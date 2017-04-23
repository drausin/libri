package routing

import (
	"container/heap"
	"errors"
	"math/big"
	"math/rand"
	"sort"
	"sync"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
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
	DefaultMaxActivePeers = uint(20)
)

// Table defines how routes to a particular target map to specific peers, held in a tree of
// buckets.
type Table interface {

	// SelfID returns the table's selfID.
	SelfID() cid.ID

	// Push adds the peer into the appropriate bucket and returns an AddStatus result.
	Push(new peer.Peer) PushStatus

	// Pop removes and returns the k peers in the bucket(s) closest to the given target.
	Pop(target cid.ID, k uint) []peer.Peer

	// Peak returns the k peers in the bucket(s) closest to the given target by popping and then
	// pushing them back into the table.
	Peak(target cid.ID, k uint) []peer.Peer

	// Get returns the peer with the given ID. It returns nil if the peer doesn't exist in the
	// table.
	Get(peerID cid.ID) peer.Peer

	// Sample returns k peers in the table sampled (approximately) uniformly from the ID space.
	// Peers are sampled from buckets with probability proportional to the amount of ID
	// space the bucket covers.
	Sample(k uint, rng *rand.Rand) []peer.Peer

	// NumPeers returns the number of total peers in the routing table.
	NumPeers() int

	// NumBuckets returns the number of buckets in the routing table.
	NumBuckets() int

	// Disconnect disconnects all client connections.
	Disconnect() error

	// Save saves the table via the NamespaceStorer
	Save(ns storage.NamespaceStorer) error
}

// Parameters are the parameters of the routing table.
type Parameters struct {

	// MaxBucketPeers is the maximum number of peers in a bucket.
	MaxBucketPeers uint
}

func NewDefaultParameters() *Parameters {
	return &Parameters{
		MaxBucketPeers: DefaultMaxActivePeers,
	}
}

type table struct {
	// this peer's node ID
	selfID cid.ID

	// all known peers, keyed by string encoding of the ID
	peers map[string]peer.Peer

	// routing buckets ordered by the max ID possible in each bucket.
	buckets []*bucket

	// defines some aspects of behavior
	params *Parameters

	// manages pushes and pops
	mu sync.Mutex
}

// NewEmpty creates a new routing table without peers.
func NewEmpty(selfID cid.ID, params *Parameters) Table {
	firstBucket := newFirstBucket(params.MaxBucketPeers)
	return &table{
		selfID:  selfID,
		peers:   make(map[string]peer.Peer),
		buckets: []*bucket{firstBucket},
		params:  params,
	}
}

// NewWithPeers creates a new routing table with peers, returning it and the number of peers added.
func NewWithPeers(selfID cid.ID, params *Parameters, peers []peer.Peer) (Table, int) {
	rt := NewEmpty(selfID, params)
	nAdded := 0
	for _, p := range peers {
		if rt.Push(p) == Added {
			nAdded++
		}
	}
	return rt, nAdded
}

// NewTestWithPeers creates a new test routing table with pseudo-random SelfID and n peers.
func NewTestWithPeers(rng *rand.Rand, n int) (Table, ecid.ID, int) {
	peerID := ecid.NewPseudoRandom(rng)
	params := NewDefaultParameters()
	rt, nAdded := NewWithPeers(peerID, params, peer.NewTestPeers(rng, n))
	return rt, peerID, nAdded
}

// SelfID returns the table's selfID.
func (rt *table) SelfID() cid.ID {
	return rt.selfID
}

func (rt *table) NumPeers() int {
	return len(rt.peers)
}

func (rt *table) NumBuckets() int {
	return rt.Len()
}

// Push adds the peer into the appropriate bucket and returns the status of the push. This method
// is concurrency-safe.
func (rt *table) Push(new peer.Peer) PushStatus {
	rt.mu.Lock()
	// get the bucket to insert into
	bucketIdx := rt.bucketIndex(new.ID())
	insertBucket := rt.buckets[bucketIdx]

	if pHeapIdx, exists := insertBucket.positions[new.ID().String()]; exists {
		// node is already in the bucket, so update it and re-heap
		existing := heap.Remove(insertBucket, pHeapIdx).(peer.Peer)
		if new != existing {
			// if the two peers aren't the same instance, merge new into existing
			if err := existing.Merge(new); err != nil {
				// should never happen
				panic(err)
			}
		}
		heap.Push(insertBucket, existing)
		rt.mu.Unlock()
		return Existed
	}
	if _, exists := rt.peers[new.ID().String()]; exists {
		// should never happen, but check just in case
		panic(errorsNew("peer should be found in its insert bucket if in peers map"))
	}

	if new.Connector() == nil {
		// don't add if doesn't have connector/public address
		rt.mu.Unlock()
		return Dropped
	}

	if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		heap.Push(insertBucket, new)
		rt.peers[new.ID().String()] = new
		rt.mu.Unlock()
		return Added
	}

	if insertBucket.containsSelf {
		// no vacancy in the bucket and it contains the self ID, so split the bucket and
		// insert via (single) recursive call
		rt.splitBucket(bucketIdx)
		rt.mu.Unlock()
		return rt.Push(new)
	}

	// no vacancy in the bucket and it doesn't contain the self ID, so just drop new peer on
	// the floor
	rt.mu.Unlock()
	return Dropped
}

// Pop removes and returns the k peers in the bucket(s) closest to the given target. This method
// is concurrency safe.
func (rt *table) Pop(target cid.ID, k uint) []peer.Peer {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if np := rt.NumPeers(); k > uint(np) {
		// if we're requesting more peers than we have, just return number we have
		k = uint(np)
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
// pushing them back into the table. This method is concurrency safe.
func (rt *table) Peak(target cid.ID, k uint) []peer.Peer {
	popped := rt.Pop(target, k)

	// add the peers back
	for _, p := range popped {
		rt.Push(p)
	}
	return popped
}

// Get returns the peer (if it exists) in the table with the given ID.
func (rt *table) Get(peerID cid.ID) peer.Peer {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if p, exists := rt.peers[peerID.String()]; exists {
		return p
	}
	return nil
}

func (rt *table) Sample(k uint, rng *rand.Rand) []peer.Peer {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// get number of peers to peak from each bucket
	bucketCounts := make([]uint, len(rt.buckets))
	for c := 0; c < int(k) && c < len(rt.peers); {
		density := rng.Float64()
		idx := rt.densityBucketIndex(density)
		if int(bucketCounts[idx]) < rt.buckets[idx].Len() {
			// if we have more peers in the bucket available for sampling; when this is
			// not the case, sample again, making the distribution of buckets only
			// approximately uniform over the ID space
			bucketCounts[idx]++
			c++
		}
	}

	// peak peers from each bucket
	sample := make([]peer.Peer, 0, k)
	for i := 0; i < len(bucketCounts); i++ {
		sample = append(sample, rt.buckets[i].Peak(bucketCounts[i])...)
	}
	return sample
}

// Disconnect disconnects all client connections. This method is thread safe.
func (rt *table) Disconnect() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
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

	panic(errorsNew("should always have either a valid forward or backward index"))
}

// bucketIndex searches for the bucket containing the given target
func (rt *table) bucketIndex(target cid.ID) int {
	return sort.Search(len(rt.buckets), func(i int) bool {
		return target.Cmp(rt.buckets[i].upperBound) < 0
	})
}

// densityBucketIndex searches for the bucket containing the given ID mass density, i.e., the
// bucket spanning the ID space covering the given [0, 1) point
func (rt *table) densityBucketIndex(idDensity float64) int {
	return sort.Search(len(rt.buckets), func(i int) bool {
		return idDensity < rt.buckets[i].idCumMass
	})
}

// splitBucket splits the bucketIdx into two and relocates the nodes appropriately
func (rt *table) splitBucket(bucketIdx int) {
	current := rt.buckets[bucketIdx]

	// define the bounds of the two new buckets from those of the current bucket
	middle := splitLowerBound(current.lowerBound, current.depth)
	newIdMass := current.idMass / 2.0

	// create the new buckets
	left := &bucket{
		depth:          current.depth + 1,
		lowerBound:     current.lowerBound,
		upperBound:     middle,
		idMass:         newIdMass,
		idCumMass:      current.idCumMass - newIdMass,
		maxActivePeers: current.maxActivePeers,
		activePeers:    make([]peer.Peer, 0),
		positions:      make(map[string]int),
	}
	left.containsSelf = left.Contains(rt.selfID)

	right := &bucket{
		depth:          current.depth + 1,
		lowerBound:     middle,
		upperBound:     current.upperBound,
		idMass:         newIdMass,
		idCumMass:      current.idCumMass,
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
