package routing

import (
	"container/heap"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestBucket_PushPop(t *testing.T) {
	for n := 1; n <= 128; n *= 2 {
		b := newFirstBucket(DefaultMaxActivePeers)
		rng := rand.New(rand.NewSource(int64(n)))
		for i, p := range peer.NewTestPeers(rng, n) {

			// simulate i successful responses from peer p so that heap ordering is well-defined
			for j := 0; j < i; j++ {
				p.Recorder().Record(peer.Response, peer.Success)
			}
			heap.Push(b, p)
		}
		prev := heap.Pop(b).(peer.Peer)
		for b.Len() > 0 {
			cur := heap.Pop(b).(peer.Peer)
			assert.True(t, prev.Before(cur))
			prev = cur
		}
	}
}

func TestBucket_Peak(t *testing.T) {
	b := newFirstBucket(DefaultMaxActivePeers)

	// nothing to peak b/c bucket is empty
	assert.Equal(t, 0, len(b.Peak(2)))

	// add some peers
	rng := rand.New(rand.NewSource(0))
	for _, p := range peer.NewTestPeers(rng, 4) {
		heap.Push(b, p)
	}

	assert.Equal(t, 2, len(b.Peak(2)))
	assert.Equal(t, 4, len(b.Peak(4)))
	assert.Equal(t, 4, len(b.Peak(8)))
}
