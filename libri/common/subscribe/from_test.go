package subscribe

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"math/rand"
	"github.com/stretchr/testify/assert"
)

func TestFrom_Fanout(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nFans := 8
	out := make(chan *api.Publication)
	fanout := make(map[int]chan *api.Publication)
	for i := 0; i < nFans; i++ {
		fanout[i] = make(chan *api.Publication, fanSlack)
	}
	f := &from{
		out: out,
		fanout: fanout,
	}

	go f.Fanout()

	// check things get fanned out as expected
	nPubs := nFans * 4
	for i := 0; i < nPubs; i++ {
		out_pub := api.NewTestPublication(rng)
		out <- out_pub
		// check that this pub appears on all fans
		for _, fan := range f.fanout {
			fan_pub := <- fan
			assert.Equal(t, out_pub, fan_pub)
		}
	}

	// close a few channels
	close(fanout[0])
	close(fanout[1])

	outPub := api.NewTestPublication(rng)
	out <- outPub
	for _, fan := range f.fanout {
		fanPub := <- fan
		assert.Equal(t, outPub, fanPub)
	}

	// check closed fans are removed from fanout
	f.mu.Lock()
	assert.Equal(t, nFans - 2, len(f.fanout))
	f.mu.Unlock()
}

func TestFrom_New(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	out := make(chan *api.Publication)
	f := NewFrom(out).(*from)
	assert.Equal(t, 0, len(f.fanout))

	go f.Fanout()

	fan1 := f.New()
	fan2 := f.New()
	assert.Equal(t, 2, len(f.fanout))

	outPub := api.NewTestPublication(rng)
	out <- outPub

	fanPub1 := <- fan1
	assert.Equal(t, outPub, fanPub1)
	fanPub2 := <- fan2
	assert.Equal(t, outPub, fanPub2)
}