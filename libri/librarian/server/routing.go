package server

import (
	"container/heap"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sort"
)

var (
	defaultMaxActivePeers = 20
)

// RoutingBucket is collection of peers stored as a heap.
type RoutingBucket struct {
	// bit depth of the bucket in the routing table/tree (i.e., the length of the bit prefix).
	Depth uint

	// (inclusive) lower bound of IDs in this bucket
	LowerBound *big.Int

	// (exclusive) upper bound of IDs in this bucket
	UpperBound *big.Int

	// maximum number of active peers for the bucket.
	MaxActivePeers int

	// active peers in the bucket.
	ActivePeers []*Peer

	// positions (i.e., indices) of each peer (keyed by ID string) in the heap.
	Positions map[string]int

	// whether the bucket contains the current node's ID.
	ContainsSelf bool
}

func (rb *RoutingBucket) Len() int {
	return len(rb.ActivePeers)
}

func (rb *RoutingBucket) Less(i, j int) bool {
	return rb.ActivePeers[i].LatestResponse.Before(rb.ActivePeers[j].LatestResponse)
}

func (rb *RoutingBucket) Swap(i, j int) {
	rb.ActivePeers[i], rb.ActivePeers[j] = rb.ActivePeers[j], rb.ActivePeers[i]
	rb.Positions[rb.ActivePeers[i].IDStr] = i
	rb.Positions[rb.ActivePeers[j].IDStr] = j
}

func (rb *RoutingBucket) Push(p interface{}) {
	rb.ActivePeers = append(rb.ActivePeers, p.(*Peer))
	rb.Positions[p.(*Peer).IDStr] = len(rb.ActivePeers) - 1
}

func (rb *RoutingBucket) Pop() interface{} {
	root := rb.ActivePeers[len(rb.ActivePeers)-1]
	rb.ActivePeers = rb.ActivePeers[0 : len(rb.ActivePeers)-1]
	delete(rb.Positions, root.IDStr)
	return root
}

// Vacancy returns whether the bucket has room for more peers.
func (rb *RoutingBucket) Vacancy() bool {
	return len(rb.ActivePeers) < rb.MaxActivePeers
}

func (rb *RoutingBucket) Contains(target *big.Int) bool {
	return target.Cmp(rb.LowerBound) >= 0 && target.Cmp(rb.UpperBound) < 0
}

// RoutingTable defines how routes to a particular target map to specific peers, held in a tree of
// RoutingBuckets.
type RoutingTable struct {
	// This peer's node ID
	SelfID *big.Int

	// All known peers, keyed by ID string
	Peers map[string]*Peer

	// Routing buckets ordered by the max ID possible in each bucket.
	Buckets []*RoutingBucket
}

// NewRoutingTable creates a new routing table without peers.
func NewEmptyRoutingTable() *RoutingTable {
	firstBucket := newFirstBucket()
	return &RoutingTable{
		Buckets: []*RoutingBucket{firstBucket},
	}
}

// NewRoutingTableWithPeers creates a new routing table from a collection of peers.
func NewRoutingTableWithPeers(peers map[string]Peer) *RoutingTable {
	rt := NewEmptyRoutingTable()
	for _, peer := range peers {
		rt.AddPeer(&peer)
	}

	return rt
}

// newFirstBucket creates a new instance of the first bucket (spanning the entire ID range)
func newFirstBucket() *RoutingBucket {
	return &RoutingBucket{
		Depth:          0,
		LowerBound:     IDLowerBound,
		UpperBound:     IDUpperBound,
		MaxActivePeers: defaultMaxActivePeers,
		ActivePeers:    make([]*Peer, 0),
		Positions:      make(map[string]int),
		ContainsSelf:   true,
	}
}

func (rt *RoutingTable) Len() int {
	return len(rt.Buckets)
}

func (rt *RoutingTable) Less(i, j int) bool {
	return rt.Buckets[i].LowerBound.Cmp(rt.Buckets[j].LowerBound) < 0
}

func (rt *RoutingTable) Swap(i, j int) {
	rt.Buckets[i], rt.Buckets[j] = rt.Buckets[j], rt.Buckets[i]
}

// NumActivePeers returns the total number of active peers across all buckets.
func (rt *RoutingTable) NumActivePeers() int {
	n := 0
	for _, rb := range rt.Buckets {
		n += rb.Len()
	}
	return n
}

// AddPeer adds the peer into the appropriate bucket and returns a tuple indicating one of the
// following outcomes:
// 	- peer already existed in bucket:	(bucket index, true, nil)
//	- peer did not exist in bucket: 	(bucket index, false, nil)
// 	- bucket was full, so peer dropped:	(-1, false, nil)
// 	- error 				(bucket index, true|false, error)
func (rt *RoutingTable) AddPeer(new *Peer) (int, bool, error) {
	// get the bucket to insert into
	bucketIdx := rt.bucketIndex(new.ID)
	insertBucket := rt.Buckets[bucketIdx]

	if pHeapIdx, ok := insertBucket.Positions[new.IDStr]; ok {
		// node is already in the bucket, so remove it and add it to the end

		existing := insertBucket.ActivePeers[pHeapIdx]
		if existing.ID.Cmp(new.ID) != 0 {
			return bucketIdx, true,
				errors.New(fmt.Sprintf("existing peer does not have same nodeId "+
					"(%s) as new peer (%s)", existing.ID, new.ID))
		}

		// move node to bottom of heap
		heap.Remove(insertBucket, pHeapIdx)
		heap.Push(insertBucket, new)

		return bucketIdx, true, nil
	}

	if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		heap.Push(insertBucket, new)

		return bucketIdx, false, nil
	}

	if insertBucket.ContainsSelf {
		// no vacancy in the bucket and it contains the self ID, so split the bucket and
		// insert via single recursive call
		err := rt.splitBucket(bucketIdx)
		if err != nil {
			return bucketIdx, false, err
		}
		return rt.AddPeer(new)

	}

	// no vacancy in the bucket and it doesn't contain the self ID, so just drop it on the floor
	return -1, false, nil
}

// PopNextPeers removes and returns the k peers in the bucket(s) closest to the given target.
func (rt *RoutingTable) PopNextPeers(target *big.Int, k int) ([]*Peer, []int, error) {
	nap := rt.NumActivePeers()
	if nap == 0 {
		return nil, nil, errors.New("No active peers exist")
	}
	if k < 1 {
		return nil, nil, errors.New(fmt.Sprintf("number of peers (n = %d) must be "+
			"positive", k))
	}
	if k > nap {
		k = nap
	}

	next := make([]*Peer, k)
	bucketIdxs := make([]int, k)

	fwdIdx := rt.bucketIndex(target)
	bkwdIdx := fwdIdx - 1

	// loop until we've populated all the peers or we have no more buckets to draw from
	for i := 0; i < k; {
		bucketIdx, err := rt.chooseBucketIndex(target, fwdIdx, bkwdIdx)
		if err != nil {
			return nil, nil, err
		}

		// fill peers from this bucket
		for ; i < k && rt.Buckets[bucketIdx].Len() > 0; i++ {
			next[i] = rt.Buckets[bucketIdx].Pop().(*Peer)
			bucketIdxs[i] = bucketIdx
		}

		// increment forward index or decrement backward index
		if bucketIdx == fwdIdx {
			fwdIdx++
		} else {
			bkwdIdx--
		}
	}

	return next, bucketIdxs, nil
}

// PopNextPeers returns (but does not remove) the n peers in the bucket(s) closest to the given
// target.
func (rt *RoutingTable) PeakNextPeers(target *big.Int, n int) ([]*Peer, []int, error) {
	nextPeers, bucketIdxs, err := rt.PopNextPeers(target, n)
	if err != nil {
		return nil, nil, err
	}

	// add the peers back into their respective buckets
	for i, nextPeer := range nextPeers {
		rt.Buckets[bucketIdxs[i]].Push(nextPeer)
	}
	return nextPeers, bucketIdxs, nil
}

// nextBucketIndex returns either the forward or backward bucket index from which to draw peers for
// a target.
func (rt *RoutingTable) chooseBucketIndex(target *big.Int, fwdIdx int, bkwdIdx int) (int, error) {
	hasFwd := fwdIdx < len(rt.Buckets)
	hasBkwd := bkwdIdx >= 0

	// determine whether to use the forward or backward index as the current index
	if hasFwd && rt.Buckets[fwdIdx].Contains(target) {
		// forward index contains target
		return fwdIdx, nil
	}

	if hasFwd && !hasBkwd {
		// have forward but no backward index
		return fwdIdx, nil
	}

	if !hasFwd && hasBkwd {
		// have backward but no forward index
		return bkwdIdx, nil
	}

	if hasFwd && hasBkwd {
		// have both backward and forward indices
		fwdDist := big.NewInt(0).Xor(target, rt.Buckets[fwdIdx].UpperBound)
		bkwdDist := big.NewInt(0).Xor(target, rt.Buckets[bkwdIdx].LowerBound)
		log.Printf("fwdist: %v, bkwdDist: %v", fwdDist, bkwdDist)

		if fwdDist.Cmp(bkwdDist) < 0 {
			// forward upper bound is closer than backward lower bound
			return fwdIdx, nil
		}

		// backward lower bound is closer than forward upper bound
		return bkwdIdx, nil
	}

	return -1, errors.New("should always have either a valid forward or backward index")
}

// bucketIndex searches for bucket containing the given target
func (rt *RoutingTable) bucketIndex(target *big.Int) int {
	return sort.Search(len(rt.Buckets), func(i int) bool {
		return target.Cmp(rt.Buckets[i].UpperBound) < 0
	})
}

// splitBucket splits the bucketIdx into two and relocates the nodes appropriately
func (rt *RoutingTable) splitBucket(bucketIdx int) error {
	current := rt.Buckets[bucketIdx]

	// define the bounds of the two new buckets from those of the current bucket
	middle := splitLowerBound(current.LowerBound, current.Depth)

	// create the new buckets
	left := &RoutingBucket{
		Depth:          current.Depth + 1,
		LowerBound:     current.LowerBound,
		UpperBound:     middle,
		MaxActivePeers: current.MaxActivePeers,
		ActivePeers:    make([]*Peer, 0),
		Positions:      make(map[string]int),
	}
	left.ContainsSelf = left.Contains(rt.SelfID)

	right := &RoutingBucket{
		Depth:          current.Depth + 1,
		LowerBound:     middle,
		UpperBound:     current.UpperBound,
		MaxActivePeers: current.MaxActivePeers,
		ActivePeers:    make([]*Peer, 0),
		Positions:      make(map[string]int),
	}
	right.ContainsSelf = right.Contains(rt.SelfID)

	// fill the buckets with existing peers
	for _, p := range current.ActivePeers {
		if left.Contains(p.ID) {
			heap.Push(left, p)
		} else {
			heap.Push(right, p)
		}
	}

	// replace the current bucket with the two new ones
	rt.Buckets[bucketIdx] = left           // replace the current bucket with left
	rt.Buckets = append(rt.Buckets, right) // right should actually be just to the right of left
	sort.Sort(rt)                          // but we let Sort handle moving it back there

	return nil
}

// splitLowerBound extends a lower bound one bit deeper with a 1 bit, thereby splitting
// the domain implied by the current lower bound and depth
// e.g.,
// 	extendLowerBound(00000000, 0) -> 10000000
// 	extendLowerBound(10000000, 1) -> 11000000
//	...
// 	extendLowerBound(11000000, 4) -> 11001000
func splitLowerBound(lowerBound *big.Int, depth uint) *big.Int {
	return big.NewInt(0).SetBit(lowerBound, int(NodeIDLength*8-depth-1), 1)
}
