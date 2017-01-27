package search

import (
	"math/big"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"container/heap"
	"fmt"
)

// PeerDistanceHeap represents a heap of peers sorted by their distance to a given target.
type PeerDistanceHeap interface {
	heap.Interface

	// SafePush pushes a peer onto the heap and subsequently removes a peer if the number of
	// peers exceeds the capacity. At capacity, these peer removals guarantee that the head
	// always becomes closer to the target or stays the same with each push.
	SafePush(peer.Peer) error

	// SafePushMany pushed an array of peers.
	SafePushMany([]peer.Peer) error

	// PeakDistance returns the distance from the root of the heap to the target.
	PeakDistance() *big.Int

	// PeakPeer returns (but does not remove) the the root of the heap.
	PeakPeer() peer.Peer

	// In returns whether a peer ID is in the heap
	In(cid.ID) bool

	// Distance returns the distance from a peer to the target.
	Distance(peer.Peer) *big.Int

	Capacity() int
}

// ClosestPeers is a min-heap of peers with the closest peer at the root.
type ClosestPeers interface {
	PeerDistanceHeap
}

// FarthestPeers is a max-heap of peers with the farthest peer at the root.
type FarthestPeers interface {
	PeerDistanceHeap
}

// closePeers represents a heap of peers sorted by closest distance to a given target.
type peerDistanceHeap struct {
	// target to compute distance to
	target cid.ID

	// peers we know about, sorted in a heap with farthest from the target at the root
	peers []peer.Peer

	// the distances of each peer to the target
	distances []*big.Int

	// set of IDStrs of the peers in the heap
	ids map[string]struct{}

	//  1: root is closest to target (min heap)
	// -1: root is farthest from target (max heap)
	sign int

	// capacity of the heap
	capacity int
}


func newClosestPeers(target cid.ID, capacity uint) ClosestPeers {
	return &peerDistanceHeap{
		target:    target,
		peers:     make([]peer.Peer, 0),
		distances: make([]*big.Int, 0),
		ids:       make(map[string]struct{}),
		sign:      1,
		capacity:  int(capacity),
	}
}

func newFarthestPeers(target cid.ID, capacity uint) FarthestPeers {
	return &peerDistanceHeap{
		target:    target,
		peers:     make([]peer.Peer, 0),
		distances: make([]*big.Int, 0),
		ids:       make(map[string]struct{}),
		sign:      -1,
		capacity: int(capacity),
	}
}

func (pdh *peerDistanceHeap) PeakDistance() *big.Int {
	return pdh.distances[0]
}

func (pdh *peerDistanceHeap) PeakPeer() peer.Peer {
	return pdh.peers[0]
}

func (pdh *peerDistanceHeap) In(id cid.ID) bool {
	_, in := pdh.ids[id.String()]
	return in
}

func (pdh *peerDistanceHeap) Distance(p peer.Peer) *big.Int {
	return p.ID().Distance(pdh.target)
}

func (pdh *peerDistanceHeap) SafePush(p peer.Peer) error {
	if pdh.In(p.(peer.Peer).ID()) {
		return fmt.Errorf("peer unexpectedly already in responded heap: %v",
			p.(peer.Peer).ID())
	}

	heap.Push(pdh, p)
	if pdh.Len() > pdh.capacity {
		if pdh.sign > 0 {
			// if min-heap, remove farthest peer from bottom
			heap.Remove(pdh, pdh.Len() - 1)
		} else {
			// if max-heap, remove farthest peer from head
			heap.Pop(pdh)
		}
	}

	return nil
}

func (pdh *peerDistanceHeap) SafePushMany(ps []peer.Peer) error {
	for _, p := range ps {
		if err := pdh.SafePush(p); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the current number of peers.
func (pdh *peerDistanceHeap) Len() int {
	return len(pdh.peers)
}

// Capacity returns the maximum number of peers allowed in the heap.
func (pdh *peerDistanceHeap) Capacity() int {
	return pdh.capacity
}

// Less returns whether peer i is closer (or farther in case of max heap) to the target than peer j.
func (pdh *peerDistanceHeap) Less(i, j int) bool {
	return less(pdh.sign, pdh.distances[i], pdh.distances[j])
}

// Swap swaps the peers in position i and j.
func (pdh *peerDistanceHeap) Swap(i, j int) {
	pdh.peers[i], pdh.peers[j] = pdh.peers[j], pdh.peers[i]
	pdh.distances[i], pdh.distances[j] = pdh.distances[j], pdh.distances[i]
}

func (pdh *peerDistanceHeap) Push(p interface{}) {
	pdh.peers = append(pdh.peers, p.(peer.Peer))
	pdh.distances = append(pdh.distances, pdh.Distance(p.(peer.Peer)))
	pdh.ids[p.(peer.Peer).ID().String()] = struct{}{}
}

func (pdh *peerDistanceHeap) Pop() interface{} {
	root := pdh.peers[len(pdh.peers)-1]
	pdh.peers = pdh.peers[0 : len(pdh.peers)-1]
	pdh.distances = pdh.distances[0 : len(pdh.distances)-1]
	delete(pdh.ids, root.ID().String())
	return root
}

func less(sign int, x, y *big.Int) bool {
	return sign*x.Cmp(y) < 0
}

