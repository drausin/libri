package routing

import (
	"container/heap"
	"errors"
	"math/big"
	"math/rand"
	"sort"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

// AddStatus indicates different outcomes when adding a peer to the routing table.
type PushStatus int

const (
	Existed PushStatus = iota
	Added
	Dropped
)

var (
	DefaultMaxActivePeers = uint(20)
)

// Table defines how routes to a particular target map to specific peers, held in a tree of
// buckets.
type Table interface {

	// SelfID returns the table's selfID.
	SelfID() cid.ID

	// Peers returns all known peers, keyed by string encoding of the ID.
	Peers() map[string]peer.Peer

	// NumPeers returns the total number of active peers across all buckets.
	NumPeers() uint

	// Push adds the peer into the appropriate bucket and returns an AddStatus result.
	Push(new peer.Peer) PushStatus

	// Pop removes and returns the k peers in the bucket(s) closest to the given target.
	Pop(target cid.ID, k uint) []peer.Peer

	// Peak returns the k peers in the bucket(s) closest to the given target by popping and then
	// pushing them back into the table.
	Peak(target cid.ID, k uint) []peer.Peer

	// Disconnect disconnects all client connections.
	Disconnect() error

	// Save saves the table via the NamespaceStorer
	Save(ns storage.NamespaceStorer) error
}

type table struct {
	// This peer's node ID
	selfID cid.ID

	// All known peers, keyed by string encoding of the ID
	peers map[string]peer.Peer

	// Routing buckets ordered by the max ID possible in each bucket.
	buckets []*bucket
}

// NewEmpty creates a new routing table without peers.
func NewEmpty(selfID cid.ID) Table {
	firstBucket := newFirstBucket()
	return &table{
		selfID:  selfID,
		peers:   make(map[string]peer.Peer),
		buckets: []*bucket{firstBucket},
	}
}

// NewWithPeers creates a new routing table with peers.
func NewWithPeers(selfID cid.ID, peers []peer.Peer) Table {
	rt := NewEmpty(selfID)
	for _, p := range peers {
		rt.Push(p)
	}
	return rt
}

// NewTestWithPeers creates a new test routing table with pseudo-random SelfID and n peers.
func NewTestWithPeers(rng *rand.Rand, n int) (Table, ecid.ID) {
	peerID := ecid.NewPseudoRandom(rng)
	return NewWithPeers(peerID, peer.NewTestPeers(rng, n)), peerID
}

// SelfID returns the table's selfID.
func (rt *table) SelfID() cid.ID {
	return rt.selfID
}

// Peers returns all known peers, keyed by string encoding of the ID.
func (rt *table) Peers() map[string]peer.Peer {
	return rt.peers
}

// NumPeers returns the total number of active peers across all buckets.
func (rt *table) NumPeers() uint {
	n := 0
	for _, rb := range rt.buckets {
		n += rb.Len()
	}
	return uint(n)
}

// Push adds the peer into the appropriate bucket and returns an AddStatus result.
func (rt *table) Push(new peer.Peer) PushStatus {
	// get the bucket to insert into
	bucketIdx := rt.bucketIndex(new.ID())
	insertBucket := rt.buckets[bucketIdx]

	if pHeapIdx, ok := insertBucket.positions[new.ID().String()]; ok {
		// node is already in the bucket, so update it and re-heap
		heap.Remove(insertBucket, pHeapIdx)
		heap.Push(insertBucket, new)
		return Existed
	}

	if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		heap.Push(insertBucket, new)
		rt.peers[new.ID().String()] = new
		return Added
	}

	if insertBucket.containsSelf {
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
func (rt *table) Pop(target cid.ID, k uint) []peer.Peer {
	if np := rt.NumPeers(); k > np {
		// if we're requesting more peers than we have, just return number we have
		k = np
	}

	fwdIdx := rt.bucketIndex(target)
	bkwdIdx := fwdIdx - 1

	// loop until we've populated all the peers or we have no more buckets to draw from
	next := make([]peer.Peer, k)
	for i := uint(0); i < k; {
		bucketIdx := rt.chooseBucketIndex(target, fwdIdx, bkwdIdx)

		// fill peers from this bucket
		for ; i < k && rt.buckets[bucketIdx].Len() > 0; i++ {
			next[i] = heap.Pop(rt.buckets[bucketIdx]).(peer.Peer)
			delete(rt.peers, next[i].ID().String())
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
func (rt *table) Peak(target cid.ID, k uint) []peer.Peer {
	popped := rt.Pop(target, k)

	// add the peers back
	for _, p := range popped {
		rt.Push(p)
	}
	return popped
}

// Disconnect disconnects all client connections.
func (rt *table) Disconnect() error {
	// disconnect from all peers
	for _, p := range rt.peers {
		if err := p.Connector().Disconnect(); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the current number of buckets in the routing table.
func (rt *table) Len() int {
	return len(rt.buckets)
}

// Less returns whether bucket i's ID lower bound is less than bucket j's.
func (rt *table) Less(i, j int) bool {
	return rt.buckets[i].Before(rt.buckets[j])
}

// Swap swaps buckets i and j.
func (rt *table) Swap(i, j int) {
	rt.buckets[i], rt.buckets[j] = rt.buckets[j], rt.buckets[i]
}

// chooseBucketIndex returns either the forward or backward bucket index from which to draw peers
// for a target.
func (rt *table) chooseBucketIndex(target cid.ID, fwdIdx int, bkwdIdx int) int {
	hasFwd := fwdIdx < len(rt.buckets)
	hasBkwd := bkwdIdx >= 0

	// determine whether to use the forward or backward index as the current index
	if hasFwd && rt.buckets[fwdIdx].Contains(target) {
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
		fwdDist := target.Distance(rt.buckets[fwdIdx].upperBound)
		bkwdDist := target.Distance(rt.buckets[bkwdIdx].lowerBound)

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
func (rt *table) bucketIndex(target cid.ID) int {
	return sort.Search(len(rt.buckets), func(i int) bool {
		return target.Cmp(rt.buckets[i].upperBound) < 0
	})
}

// splitBucket splits the bucketIdx into two and relocates the nodes appropriately
func (rt *table) splitBucket(bucketIdx int) {
	current := rt.buckets[bucketIdx]

	// define the bounds of the two new buckets from those of the current bucket
	middle := splitLowerBound(current.lowerBound, current.depth)

	// create the new buckets
	left := &bucket{
		depth:          current.depth + 1,
		lowerBound:     current.lowerBound,
		upperBound:     middle,
		maxActivePeers: current.maxActivePeers,
		activePeers:    make([]peer.Peer, 0),
		positions:      make(map[string]int),
	}
	left.containsSelf = left.Contains(rt.selfID)

	right := &bucket{
		depth:          current.depth + 1,
		lowerBound:     middle,
		upperBound:     current.upperBound,
		maxActivePeers: current.maxActivePeers,
		activePeers:    make([]peer.Peer, 0),
		positions:      make(map[string]int),
	}
	right.containsSelf = right.Contains(rt.selfID)

	// fill the buckets with existing peers
	for _, p := range current.activePeers {
		if left.Contains(p.ID()) {
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
func splitLowerBound(lowerBound cid.ID, depth uint) cid.ID {
	return cid.FromInt(new(big.Int).SetBit(lowerBound.Int(), int(cid.Length*8-depth-1), 1))
}
