package routing

import (
	"container/heap"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

// TODO: add unit tests for all bucket methods

func TestBucket_PushPop(t *testing.T) {
	for n := 1; n <= 128; n *= 2 {
		b := newFirstBucket()
		rng := rand.New(rand.NewSource(int64(n)))
		for _, p := range peer.NewTestPeers(rng, n) {
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
