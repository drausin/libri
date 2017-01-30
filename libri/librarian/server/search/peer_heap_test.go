package search

import (
	"container/heap"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestClosestPeers_Heap(t *testing.T) {
	testPeerDistanceHeap(t, true)
}

func TestFarthestPeers_Heap(t *testing.T) {
	testPeerDistanceHeap(t, false)
}

func TestPeerDistanceHeap_Distance(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := cid.NewPseudoRandom(rng)
	cp := newClosestPeers(target, 8)
	for _, p := range peer.NewTestPeers(rng, 8) {
		assert.True(t, target.Distance(p.ID()).Cmp(cp.Distance(p)) == 0)
	}
}

func TestPeerDistanceHeap_In(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := cid.NewPseudoRandom(rng)
	cp := newClosestPeers(target, 8)
	for _, p := range peer.NewTestPeers(rng, 8) {
		assert.False(t, cp.In(p.ID()))
		assert.Nil(t, cp.SafePush(p))
		assert.True(t, cp.In(p.ID()))
	}
}

func TestPeerDistanceHeap_SafePush_exists(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := cid.NewPseudoRandom(rng)
	cp := newClosestPeers(target, 8)
	p := peer.NewTestPeer(rng, 0)
	assert.Nil(t, cp.SafePush(p))
	assert.NotNil(t, cp.SafePush(p)) // should break b/c p0 already in there
}

func testPeerDistanceHeap(t *testing.T, minHeap bool) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := cid.NewPseudoRandom(rng)
	var cp PeerDistanceHeap
	if minHeap {
		cp = newClosestPeers(target, 20)
	} else {
		cp = newFarthestPeers(target, 20)
	}

	// add the peers
	assert.Nil(t, cp.SafePushMany(peer.NewTestPeers(rng, 20)))

	// make sure our peak methods behave as expected
	curDist := cp.PeakDistance()
	assert.Equal(t, curDist, target.Distance(cp.PeakPeer().ID()))
	assert.Equal(t, curDist, cp.Distance(cp.PeakPeer()))
	heap.Pop(cp)

	for ; cp.Len() > 0; heap.Pop(cp) {
		if minHeap {
			// each subsequent peer should be farther away
			assert.True(t, curDist.Cmp(cp.PeakDistance()) < 0)
		} else {
			// each subsequent peer should be closer
			assert.True(t, curDist.Cmp(cp.PeakDistance()) > 0)
		}
		curDist = cp.PeakDistance()
	}
}

func TestClosestPeers_SafePush_capacity(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := cid.NewPseudoRandom(rng)
	n := 8
	testPeerDistanceHeapSafePushCapacity(t, rng, newClosestPeers(target, uint(n)))
}

func TestFarthestPeers_SafePush_capacity(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := cid.NewPseudoRandom(rng)
	n := 8
	testPeerDistanceHeapSafePushCapacity(t, rng, newFarthestPeers(target, uint(n)))
}

func testPeerDistanceHeapSafePushCapacity(t *testing.T, rng *rand.Rand, pdh PeerDistanceHeap) {
	for _, p := range peer.NewTestPeers(rng, pdh.Capacity()) {
		assert.True(t, pdh.Capacity() > pdh.Len())
		assert.Nil(t, pdh.SafePush(p))
	}
	assert.Equal(t, pdh.Capacity(), pdh.Len())

	prevDistance := pdh.PeakDistance()
	for _, p := range peer.NewTestPeers(rng, pdh.Capacity()) {
		assert.Nil(t, pdh.SafePush(p))

		// length should stay the same since we're at capacity
		assert.Equal(t, pdh.Capacity(), pdh.Len())

		// root distance should always be less than previous distance
		assert.True(t, pdh.PeakDistance().Cmp(prevDistance) <= 0)
		prevDistance = pdh.PeakDistance()
	}
}
