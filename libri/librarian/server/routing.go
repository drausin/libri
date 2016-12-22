package server

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"sort"
)

var (
	defaultMaxActivePeers = 30
)

// RoutingBucket is collection of peers stored as a heap.
type RoutingBucket struct {
	// The bit depth of the bucket in the routing table/tree (i.e., the length of the bit prefix).
	Depth uint

	// The (inclusive) lower bound of IDs in this bucket
	LowerBound []byte

	// THE (exclusive) upper bound of IDs in this bucket
	UpperBound []byte

	// The maximum number of active peers for the bucket.
	MaxActivePeers int

	// The active peers in the bucket.
	ActivePeers []*Peer

	// The positions (i.e., indices) of each peer (by string ID) in the heap.
	Positions map[string]int

	// Whether the bucket contains the current node's ID.
	ContainsSelf bool
}

func (rb *RoutingBucket) Len() int {
	return len(rb.ActivePeers)
}

func (rb *RoutingBucket) Less(i, j int) bool {
	return rb.ActivePeers[i].LatestResponse < rb.ActivePeers[j].LatestResponse
}

func (rb *RoutingBucket) Swap(i, j int) {
	rb.ActivePeers[i], rb.ActivePeers[j] = rb.ActivePeers[j], rb.ActivePeers[i]
	rb.Positions[rb.ActivePeers[i].PeerIdStr] = i
	rb.Positions[rb.ActivePeers[j].PeerIdStr] = j
}

func (rb *RoutingBucket) Push(p interface{}) {
	rb.ActivePeers = append(rb.ActivePeers, p.(*Peer))
	rb.Positions[p.(*Peer).PeerIdStr] = len(rb.ActivePeers) - 1
}

func (rb *RoutingBucket) Pop() interface{} {
	root := rb.ActivePeers[len(rb.ActivePeers)-1]
	rb.ActivePeers = rb.ActivePeers[0 : len(rb.ActivePeers)-1]
	delete(rb.Positions, root.PeerIdStr)
	return root
}

// Vacancy returns whether the bucket has room for more peers.
func (rb *RoutingBucket) Vacancy() bool {
	return len(rb.ActivePeers) < rb.MaxActivePeers
}

func (rb *RoutingBucket) Contains(id []byte) bool {
	return bytes.Compare(id, rb.LowerBound) >= 0 && bytes.Compare(id, rb.UpperBound) < 0
}

// RoutingTable defines how routes to a particular target map to specific peers, held in a tree of RoutingBuckets.
type RoutingTable struct {
	// This peer's node ID
	SelfID []byte

	// All known peers, key by the string representation of their node ID.
	peers map[string]Peer

	// Routing buckets ordered by the max ID possible in each bucket.
	buckets []*RoutingBucket
}

// NewRoutingTable creates a new routing table from a collection of peers.
func NewRoutingTable(peers map[string]Peer) *RoutingTable {
	firstBucket := RoutingBucket{
		Depth:          0,
		LowerBound:     make([]byte, NodeIDLength),
		UpperBound:     bytes.Repeat([]byte{255}, NodeIDLength),
		MaxActivePeers: defaultMaxActivePeers,
		ContainsSelf:   true,
	}
	rt := RoutingTable{
		peers:   peers,
		buckets: []*RoutingBucket{&firstBucket},
	}

	for _, peer := range peers {
		rt.AddPeer(&peer)
	}

	return &rt
}

func (rt *RoutingTable) Len() int {
	return len(rt.buckets)
}

func (rt *RoutingTable) Less(i, j int) bool {
	return bytes.Compare(rt.buckets[i].LowerBound, rt.buckets[i].LowerBound) < 0
}

func (rt *RoutingTable) Swap(i, j int) {
	rt.buckets[i], rt.buckets[j] = rt.buckets[j], rt.buckets[i]
}

// AddPeer adds the peer into the appropriate bucket.
func (rt *RoutingTable) AddPeer(new *Peer) {
	// get the bucket to insert into
	bucketIdx := rt.bucketIndex(new.PeerId)
	insertBucket := rt.buckets[bucketIdx]

	if pHeapIdx, ok := insertBucket.Positions[new.PeerIdStr]; ok {
		// node is already in the bucket, so remove it and add it to the end

		existing := insertBucket.ActivePeers[pHeapIdx]
		if !bytes.Equal(existing.PeerId, new.PeerId) {
			panic(errors.New(fmt.Sprintf("existing peer does not have same nodeId (%s) as new peer (%s)",
				existing.PeerId, new.PeerId)))
		}

		// move node to bottom of heap
		heap.Remove(insertBucket, pHeapIdx)
		heap.Push(insertBucket, new)

	} else if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		heap.Push(insertBucket, new)

	} else if insertBucket.ContainsSelf {
		// no vacancy in the bucket and it contains the self ID, so split the bucket and insert via single
		// recursive call
		rt.splitBucket(bucketIdx)
		rt.AddPeer(new)

	} else {
		// no vacancy in the bucket and it doesn't contain the self ID, so just drop it on the floor
	}
}

// NextPeers returns the next n peers in the same bucket as the given target.
func (rt *RoutingTable) NextPeers(target []byte, n int) []*Peer {
	next := make([]*Peer, n)
	bi := rt.bucketIndex(target)
	for i := 0; i < n; i++ {
		next[i] = rt.buckets[bi].Pop().(*Peer)
	}
	return next
}

// bucketIndex searches for bucket containing the given target
func (rt *RoutingTable) bucketIndex(target []byte) int {
	return sort.Search(len(rt.buckets), func(i int) bool {
		return bytes.Compare(target, rt.buckets[i].UpperBound) < 0
	})
}

// splitBucket splits the bucketIdx into two and relocates the nodes appropriately
func (rt *RoutingTable) splitBucket(bucketIdx int) {
	current := rt.buckets[bucketIdx]

	// define the bounds of the two new buckets from those of the current bucket
	middle := splitLowerBound(current.LowerBound, current.Depth)

	// create the new buckets
	left := &RoutingBucket{
		Depth:          current.Depth + 1,
		LowerBound:     current.LowerBound,
		UpperBound:     middle,
		MaxActivePeers: current.MaxActivePeers,
	}
	left.ContainsSelf = left.Contains(rt.SelfID)

	right := &RoutingBucket{
		Depth:          current.Depth + 1,
		LowerBound:     middle,
		UpperBound:     current.UpperBound,
		MaxActivePeers: current.MaxActivePeers,
	}
	right.ContainsSelf = !left.ContainsSelf

	// fill the buckets with existing peers
	for _, p := range current.ActivePeers {
		if left.Contains(p.PeerId) {
			heap.Push(left, p)
		} else {
			heap.Push(right, p)
		}
	}

	// replace the current bucket with the two new ones
	rt.buckets[bucketIdx] = left           // replace the current bucket with left
	rt.buckets = append(rt.buckets, right) // right should actually be just to the right of left
	sort.Sort(rt)                          // but we let Sort handle moving it back there
}

// splitLowerBound extends a lower bound one bit deeper with a 1 bit, thereby splitting
// the domain implied by the current lower bound and depth
// e.g.,
// 	extendLowerBound(00000000, 0) -> 10000000
// 	extendLowerBound(10000000, 1) -> 11000000
//	...
// 	extendLowerBound(11000000, 4) -> 11001000
func splitLowerBound(current []byte, depth uint) []byte {

	split := make([]byte, len(current))
	b := uint(0)
	for ; b*8 < depth; b++ {
		split[b] = current[b]
	}
	split[b] = current[b] | 1<<(7-depth)

	return split
}
