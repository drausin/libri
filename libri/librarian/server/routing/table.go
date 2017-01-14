package routing

import (
	"container/heap"
	"errors"
	"math/big"
	"math/rand"
	"sort"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

// AddStatus indicates different outcomes when adding a peer to the routing table.
type PushStatus int

const (
	Existed PushStatus = iota
	Added
	Dropped
)

var (
	DefaultMaxActivePeers = 20
)

// TODO (drausin): replace public struct w/ interface + private struct

// Table defines how routes to a particular target map to specific peers, held in a tree of
// buckets.
type Table struct {
	// This peer's node ID
	SelfID *big.Int

	// All known peers, keyed by string encoding of the ID
	Peers map[string]*peer.Peer

	// Routing buckets ordered by the max ID possible in each bucket.
	Buckets []*bucket
}

// NewEmpty creates a new routing table without peers.
func NewEmpty(selfID *big.Int) *Table {
	firstBucket := newFirstBucket()
	return &Table{
		SelfID:  selfID,
		Peers:   make(map[string]*peer.Peer),
		Buckets: []*bucket{firstBucket},
	}
}

// NewWithPeers creates a new routing table with peers.
func NewWithPeers(selfID *big.Int, peers []*peer.Peer) *Table {
	rt := NewEmpty(selfID)
	for _, p := range peers {
		rt.Push(p)
	}
	return rt
}

// NewTestWithPeers creates a new test routing table with pseudo-random SelfID and n peers.
func NewTestWithPeers(rng *rand.Rand, n int) *Table {
	return NewWithPeers(id.NewPseudoRandom(rng), peer.NewTestPeers(rng, n))
}

// NumPeers returns the total number of active peers across all buckets.
func (rt *Table) NumPeers() uint {
	n := 0
	for _, rb := range rt.Buckets {
		n += rb.Len()
	}
	return uint(n)
}

// Close disconnects all client connections and saves the routing table state to the DB.
func (rt *Table) Close(db db.KVDB) error {
	// disconnect from all peers
	for _, p := range rt.Peers {
		if err := p.Disconnect(); err != nil {
			return err
		}
	}
	return nil
}

// Push adds the peer into the appropriate bucket and returns an AddStatus result.
func (rt *Table) Push(new *peer.Peer) PushStatus {
	// get the bucket to insert into
	bucketIdx := rt.bucketIndex(new.ID)
	insertBucket := rt.Buckets[bucketIdx]

	if pHeapIdx, ok := insertBucket.Positions[new.IDStr]; ok {
		// node is already in the bucket, so update it and re-heap
		heap.Remove(insertBucket, pHeapIdx)
		heap.Push(insertBucket, new)
		return Existed
	}

	if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		heap.Push(insertBucket, new)
		rt.Peers[new.IDStr] = new
		return Added
	}

	if insertBucket.ContainsSelf {
		// no vacancy in the bucket and it contains the self ID, so split the bucket and
		// insert via (single) recursive call
		rt.splitBucket(bucketIdx)
		return rt.Push(new)
	}

	// no vacancy in the bucket and it doesn't contain the self ID, so just drop new peer on
	// the floor
	return Dropped
}

// Pop removes and returns the k peers in the bucket(s) closest to the given target.
func (rt *Table) Pop(target *big.Int, k uint) []*peer.Peer {
	if np := rt.NumPeers(); k > np {
		// if we're requesting more peers than we have, just return number we have
		k = np
	}

	fwdIdx := rt.bucketIndex(target)
	bkwdIdx := fwdIdx - 1

	// loop until we've populated all the peers or we have no more buckets to draw from
	next := make([]*peer.Peer, k)
	for i := uint(0); i < k; {
		bucketIdx := rt.chooseBucketIndex(target, fwdIdx, bkwdIdx)

		// fill peers from this bucket
		for ; i < k && rt.Buckets[bucketIdx].Len() > 0; i++ {
			next[i] = heap.Pop(rt.Buckets[bucketIdx]).(*peer.Peer)
			delete(rt.Peers, next[i].IDStr)
		}

		// (in|de)crement the appropriate index
		if bucketIdx == fwdIdx {
			fwdIdx++
		} else {
			bkwdIdx--
		}
	}

	return next
}

// Peak returns the k peers in the bucket(s) closest to the given target by popping and then
// pushing them back into the table.
func (rt *Table) Peak(target *big.Int, k uint) []*peer.Peer {
	popped := rt.Pop(target, k)

	// add the peers back
	for _, p := range popped {
		rt.Push(p)
	}
	return popped
}

// Len returns the current number of buckets in the routing table.
func (rt *Table) Len() int {
	return len(rt.Buckets)
}

// Less returns whether bucket i's ID lower bound is less than bucket j's.
func (rt *Table) Less(i, j int) bool {
	return rt.Buckets[i].Before(rt.Buckets[j])
}

// Swap swaps buckets i and j.
func (rt *Table) Swap(i, j int) {
	rt.Buckets[i], rt.Buckets[j] = rt.Buckets[j], rt.Buckets[i]
}

// chooseBucketIndex returns either the forward or backward bucket index from which to draw peers
// for a target.
func (rt *Table) chooseBucketIndex(target *big.Int, fwdIdx int, bkwdIdx int) int {
	hasFwd := fwdIdx < len(rt.Buckets)
	hasBkwd := bkwdIdx >= 0

	// determine whether to use the forward or backward index as the current index
	if hasFwd && rt.Buckets[fwdIdx].Contains(target) {
		// forward index contains target
		return fwdIdx
	}

	if hasFwd && !hasBkwd {
		// have forward but no backward index
		return fwdIdx
	}

	if !hasFwd && hasBkwd {
		// have backward but no forward index
		return bkwdIdx
	}

	if hasFwd && hasBkwd {
		// have both backward and forward indices
		fwdDist := id.Distance(target, rt.Buckets[fwdIdx].UpperBound)
		bkwdDist := id.Distance(target, rt.Buckets[bkwdIdx].LowerBound)

		if fwdDist.Cmp(bkwdDist) < 0 {
			// forward upper bound is closer than backward lower bound
			return fwdIdx
		}

		// backward lower bound is closer than forward upper bound
		return bkwdIdx
	}

	panic(errors.New("should always have either a valid forward or backward index"))
}

// bucketIndex searches for bucket containing the given target
func (rt *Table) bucketIndex(target *big.Int) int {
	return sort.Search(len(rt.Buckets), func(i int) bool {
		return target.Cmp(rt.Buckets[i].UpperBound) < 0
	})
}

// splitBucket splits the bucketIdx into two and relocates the nodes appropriately
func (rt *Table) splitBucket(bucketIdx int) {
	current := rt.Buckets[bucketIdx]

	// define the bounds of the two new buckets from those of the current bucket
	middle := splitLowerBound(current.LowerBound, current.Depth)

	// create the new buckets
	left := &bucket{
		Depth:          current.Depth + 1,
		LowerBound:     current.LowerBound,
		UpperBound:     middle,
		MaxActivePeers: current.MaxActivePeers,
		ActivePeers:    make([]*peer.Peer, 0),
		Positions:      make(map[string]int),
	}
	left.ContainsSelf = left.Contains(rt.SelfID)

	right := &bucket{
		Depth:          current.Depth + 1,
		LowerBound:     middle,
		UpperBound:     current.UpperBound,
		MaxActivePeers: current.MaxActivePeers,
		ActivePeers:    make([]*peer.Peer, 0),
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
}

// splitLowerBound extends a lower bound one bit deeper with a 1 bit, thereby splitting
// the domain implied by the current lower bound and depth
// e.g.,
// 	extendLowerBound(00000000, 0) -> 10000000
// 	extendLowerBound(10000000, 1) -> 11000000
//	...
// 	extendLowerBound(11000000, 4) -> 11001000
func splitLowerBound(lowerBound *big.Int, depth uint) *big.Int {
	return big.NewInt(0).SetBit(lowerBound, int(id.Length*8-depth-1), 1)
}
