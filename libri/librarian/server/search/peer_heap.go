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

func (cp *peerDistanceHeap) PeakDistance() *big.Int {
	return cp.distances[0]
}

func (cp *peerDistanceHeap) PeakPeer() peer.Peer {
	return cp.peers[0]
}

func (cp *peerDistanceHeap) In(id cid.ID) bool {
	_, in := cp.ids[id.String()]
	return in
}

func (cp *peerDistanceHeap) Distance(p peer.Peer) *big.Int {
	return p.ID().Distance(cp.target)
}

func (cp *peerDistanceHeap) SafePush(p peer.Peer) error {
	if cp.In(p.(peer.Peer).ID()) {
		return fmt.Errorf("peer unexpectedly already in responded heap: %v",
			p.(peer.Peer).ID())
	}

	heap.Push(cp, p)
	if cp.Len() > cp.capacity {
		if cp.sign > 0 {
			// if min-heap, remove farthest peer from bottom
			heap.Remove(cp, cp.Len() - 1)
		} else {
			// if max-heap, remove farthest peer from head
			heap.Pop(cp)
		}
	}

	return nil
}

func (cp *peerDistanceHeap) SafePushMany(ps []peer.Peer) error {
	for _, p := range ps {
		if err := cp.SafePush(p); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the current number of peers.
func (cp *peerDistanceHeap) Len() int {
	return len(cp.peers)
}

// Capacity returns the maximum number of peers allowed in the heap.
func (cp *peerDistanceHeap) Capacity() int {
	return cp.capacity
}

// Less returns whether peer i is closer (or farther in case of max heap) to the target than peer j.
func (cp *peerDistanceHeap) Less(i, j int) bool {
	return less(cp.sign, cp.distances[i], cp.distances[j])
}

// Swap swaps the peers in position i and j.
func (cp *peerDistanceHeap) Swap(i, j int) {
	cp.peers[i], cp.peers[j] = cp.peers[j], cp.peers[i]
	cp.distances[i], cp.distances[j] = cp.distances[j], cp.distances[i]
}

func (cp *peerDistanceHeap) Push(p interface{}) {
	cp.peers = append(cp.peers, p.(peer.Peer))
	cp.distances = append(cp.distances, cp.Distance(p.(peer.Peer)))
	cp.ids[p.(peer.Peer).ID().String()] = struct{}{}
}

func (cp *peerDistanceHeap) Pop() interface{} {
	root := cp.peers[len(cp.peers)-1]
	cp.peers = cp.peers[0 : len(cp.peers)-1]
	cp.distances = cp.distances[0 : len(cp.distances)-1]
	delete(cp.ids, root.ID().String())
	return root
}

func less(sign int, x, y *big.Int) bool {
	return sign*x.Cmp(y) < 0
}

