package subscribe

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestFrom_Fanout(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nFans := 8
	params := NewDefaultFromParameters()
	params.EndSubscriptionProb = 0.0 // never end
	out := make(chan *KeyedPub)
	lg := clogging.NewDevInfoLogger()
	f := NewFrom(params, lg, out).(*from)

	go f.Fanout()

	fanout := make(map[uint64]chan *KeyedPub)
	done := make(map[uint64]chan struct{})
	for i := uint64(0); int(i) < nFans; i++ {
		f, d, err := f.New()
		assert.Nil(t, err)
		fanout[i], done[i] = f, d
	}

	// check things get fanned out as expected
	nPubs := int(nFans) * 4
	for i := 0; i < nPubs; i++ {
		outPub := newKeyedPub(t, api.NewTestPublication(rng))
		out <- outPub
		// check that this pub appears on all fans
		for i := range fanout {
			fanPub := <-fanout[i]
			assert.Equal(t, outPub, fanPub)
		}
	}
}

func TestFrom_Fanout_close(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nFans := 8
	params := NewDefaultFromParameters()
	params.EndSubscriptionProb = 0.0 // never end
	out := make(chan *KeyedPub)
	lg := clogging.NewDevInfoLogger()
	f := NewFrom(params, lg, out).(*from)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		f.Fanout()

		// check fanouts have been deleted
		assert.Equal(t, 0, len(f.fanout))
		assert.Equal(t, 0, len(f.done))
	}(wg)

	fanout := make(map[uint64]chan *KeyedPub)
	done := make(map[uint64]chan struct{})
	for i := uint64(0); int(i) < nFans; i++ {
		f, d, err := f.New()
		assert.Nil(t, err)
		fanout[i], done[i] = f, d
	}

	// close a few channels
	close(done[0])
	close(done[1])
	nDeleted := 2

	outPub := newKeyedPub(t, api.NewTestPublication(rng))
	out <- outPub

	fanPub, open := <-fanout[0]
	if open { // delete didn't happen in time
		assert.Equal(t, outPub, fanPub)
		nDeleted--
	}
	fanPub, open = <-fanout[1]
	if open { // delete didn't happen in time
		assert.Equal(t, outPub, fanPub)
		nDeleted--
	}

	for i := range fanout {
		if i == 0 || i == 1 {
			continue
		}
		info := fmt.Sprintf("fan %d", i)
		fanPub := <-fanout[i]
		assert.Equal(t, outPub, fanPub, info)
	}

	// check closed fans are removed from fanout
	f.mu.Lock()
	assert.Equal(t, nFans-nDeleted, len(f.fanout))
	f.mu.Unlock()

	// ensure closing output channel closes all fanouts as well
	close(out)
	wg.Wait()
	for i := range fanout {
		pub, open := <-fanout[i]
		assert.Nil(t, pub)
		assert.False(t, open)

		_, open = <-done[i]
		assert.False(t, open)
	}
}

func TestFrom_Fanout_end(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := NewDefaultFromParameters()
	params.EndSubscriptionProb = 1.0 // always end
	out := make(chan *KeyedPub)
	lg := clogging.NewDevInfoLogger()
	f := NewFrom(params, lg, out).(*from)

	go f.Fanout()
	fanout, done, err := f.New()
	assert.Nil(t, err)

	outPub := newKeyedPub(t, api.NewTestPublication(rng))
	out <- outPub

	// check that subscription has ended and done channel is closed
	fanPub, open := <-fanout
	assert.Nil(t, fanPub)
	assert.False(t, open)
	<-done // if this doesn't block, means done is closed
}

func TestFrom_New_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := NewDefaultFromParameters()
	out := make(chan *KeyedPub)
	lg := clogging.NewDevInfoLogger()
	f := NewFrom(params, lg, out).(*from)
	assert.Equal(t, 0, len(f.fanout))

	go f.Fanout()

	fan1, _, err := f.New()
	assert.Nil(t, err)
	fan2, _, err := f.New()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(f.fanout))

	outPub := newKeyedPub(t, api.NewTestPublication(rng))
	out <- outPub

	fanPub1 := <-fan1
	assert.Equal(t, outPub, fanPub1)
	fanPub2 := <-fan2
	assert.Equal(t, outPub, fanPub2)
}

func TestFrom_New_err(t *testing.T) {
	params := &FromParameters{
		NMaxSubscriptions: 0,
	}
	out := make(chan *KeyedPub)
	lg := clogging.NewDevInfoLogger()
	f := NewFrom(params, lg, out).(*from)
	fan, done, err := f.New()
	assert.Equal(t, ErrNotAcceptingNewSubscriptions, err)
	assert.Nil(t, fan)
	assert.Nil(t, done)
}

func newKeyedPub(t *testing.T, pub *api.Publication) *KeyedPub {
	key, err := api.GetKey(pub)
	assert.Nil(t, err)
	return &KeyedPub{
		Key:   key,
		Value: pub,
	}
}
