package subscribe

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"math/rand"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestFrom_Fanout(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nFans := 8
	out := make(chan *api.Publication)
	f := NewFrom(out).(*from)

	go f.Fanout()
	fanout := make(map[uint64]chan *api.Publication)
	done := make(map[uint64]chan struct{})
	for i := uint64(0); int(i) < nFans; i++ {
		f, d := f.New()
		fanout[i], done[i] = f, d
	}

	// check things get fanned out as expected
	nPubs := int(nFans) * 4
	for i := 0; i < nPubs; i++ {
		outPub := api.NewTestPublication(rng)
		out <- outPub
		// check that this pub appears on all fans
		for i := range fanout {
			fanPub := <- fanout[i]
			assert.Equal(t, outPub, fanPub)
		}
	}

	// close a few channels
	close(done[0])
	close(done[1])
	nDeleted := 2

	outPub := api.NewTestPublication(rng)
	out <- outPub

	fanPub, open := <- fanout[0]
	if open { // delete didn't happen in time
		assert.Equal(t, outPub, fanPub)
		nDeleted--
	}
	fanPub, open = <- fanout[1]
	if open { // delete didn't happen in time
		assert.Equal(t, outPub, fanPub)
		nDeleted--
	}

	for i := range fanout {
		if i == 0 || i == 1 {
			continue
		}
		info := fmt.Sprintf("fan %d", i)
		fanPub := <- fanout[i]
		assert.Equal(t, outPub, fanPub, info)
	}

	// check closed fans are removed from fanout
	f.mu.Lock()
	assert.Equal(t, nFans - nDeleted, len(f.fanout))
	f.mu.Unlock()
}

func TestFrom_New(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	out := make(chan *api.Publication)
	f := NewFrom(out).(*from)
	assert.Equal(t, 0, len(f.fanout))

	go f.Fanout()

	fan1, _ := f.New()
	fan2, _ := f.New()
	assert.Equal(t, 2, len(f.fanout))

	outPub := api.NewTestPublication(rng)
	out <- outPub

	fanPub1 := <- fan1
	assert.Equal(t, outPub, fanPub1)
	fanPub2 := <- fan2
	assert.Equal(t, outPub, fanPub2)
}