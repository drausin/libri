package search

import (
	"container/heap"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestClosestPeers_Heap(t *testing.T) {
	testPeerDistanceHeap(t, true)
}

func TestFarthestPeers_Heap(t *testing.T) {
	testPeerDistanceHeap(t, false)
}

func TestPeerDistanceHeap_ToAPI(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	cp := NewClosestPeers(target, 8)
	addresses := cp.ToAPI()
	assert.Equal(t, cp.Len(), len(addresses))
	for _, a := range addresses {
		assert.True(t, cp.In(id.FromBytes(a.PeerId)))
	}
}

func TestPeerDistanceHeap_Distance(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	cp := NewClosestPeers(target, 8)
	for _, p := range peer.NewTestPeers(rng, 8) {
		assert.True(t, target.Distance(p.ID()).Cmp(cp.Distance(p)) == 0)
	}
}

func TestPeerDistanceHeap_In(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	cp := NewClosestPeers(target, 8)
	for _, p := range peer.NewTestPeers(rng, 8) {
		assert.False(t, cp.In(p.ID()))
		cp.SafePush(p)
		assert.True(t, cp.In(p.ID()))
	}
}

func TestPeerDistanceHeap_SafePush_exists(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	cp := NewClosestPeers(target, 8)
	p := peer.NewTestPeer(rng, 0)
	cp.SafePush(p)
	cp.SafePush(p) // no-op
	assert.Equal(t, 1, cp.Len())
}

func TestPeerDistanceHeap_Peers(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	n := 8
	pdh := NewFarthestPeers(target, uint(n))
	for _, p := range peer.NewTestPeers(rng, 8) {
		pdh.SafePush(p)
	}
	ordered := pdh.Peers()

	// each peer should be closer to target than the previous
	for i := 1; i < len(ordered); i++ {
		assert.NotNil(t, ordered[i-1])
		assert.NotNil(t, ordered[i])
		p1Dist, p2Dist := pdh.Distance(ordered[i-1]), pdh.Distance(ordered[i])
		assert.True(t, p1Dist.Cmp(p2Dist) > 0)
	}
}

func testPeerDistanceHeap(t *testing.T, minHeap bool) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	var cp PeerDistanceHeap
	if minHeap {
		cp = NewClosestPeers(target, 20)
	} else {
		cp = NewFarthestPeers(target, 20)
	}

	// add the peers
	cp.SafePushMany(peer.NewTestPeers(rng, 20))

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
	target := id.NewPseudoRandom(rng)
	n := 8
	testPeerDistanceHeapSafePushCapacity(t, rng, NewClosestPeers(target, uint(n)))
}

func TestFarthestPeers_SafePush_capacity(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	n := 8
	testPeerDistanceHeapSafePushCapacity(t, rng, NewFarthestPeers(target, uint(n)))
}

func testPeerDistanceHeapSafePushCapacity(t *testing.T, rng *rand.Rand, pdh PeerDistanceHeap) {
	for _, p := range peer.NewTestPeers(rng, pdh.Capacity()) {
		assert.True(t, pdh.Capacity() > pdh.Len())
		pdh.SafePush(p)
	}
	assert.Equal(t, pdh.Capacity(), pdh.Len())

	prevDistance := pdh.PeakDistance()
	for _, p := range peer.NewTestPeers(rng, pdh.Capacity()) {
		pdh.SafePush(p)

		// length should stay the same since we're at capacity
		assert.Equal(t, pdh.Capacity(), pdh.Len())

		// root distance should always be less than previous distance
		assert.True(t, pdh.PeakDistance().Cmp(prevDistance) <= 0)
		prevDistance = pdh.PeakDistance()
	}
}
