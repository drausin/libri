package routing

import (
	"container/heap"
	"errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"math/big"
	"sort"
)

// AddStatus indicates different outcomes when adding a peer to the routing table.
type PushStatus int

const (
	Existed PushStatus = iota
	Added
	Dropped
)

var (
	defaultMaxActivePeers = 20
)

// RoutingTable defines how routes to a particular target map to specific peers, held in a tree of
// RoutingBuckets.
type RoutingTable struct {
	// This peer's node ID
	SelfID *big.Int

	// All known peers, keyed by ID
	Peers map[big.Int]*peer.Peer

	// Routing buckets ordered by the max ID possible in each bucket.
	Buckets []*routingBucket
}

// NewEmpty creates a new routing table without peers.
func NewEmpty(selfID *big.Int) *RoutingTable {
	firstBucket := newFirstBucket()
	return &RoutingTable{
		SelfID:  selfID,
		Peers:   make(map[big.Int]*peer.Peer),
		Buckets: []*routingBucket{firstBucket},
	}
}

// NewWithPeers creates a new routing table with peers.
func NewWithPeers(selfID *big.Int, peers []*peer.Peer) *RoutingTable {
	rt := NewEmpty(selfID)
	for _, p := range peers {
		rt.Push(p)
	}
	return rt
}


// NumPeers returns the total number of active peers across all buckets.
func (rt *RoutingTable) NumPeers() int {
	n := 0
	for _, rb := range rt.Buckets {
		n += rb.Len()
	}
	return n
}

// Close disconnects all client connections and saves the routing table state to the DB.
func (rt *RoutingTable) Close(db db.KVDB) error {
	// disconnect from all peers
	for _, p := range rt.Peers {
		if err := p.Disconnect(); err != nil {
			return err
		}
	}

	// save to DB
	return rt.Save(db)
}

// Push adds the peer into the appropriate bucket and returns an AddStatus result.
func (rt *RoutingTable) Push(new *peer.Peer) PushStatus {
	// get the bucket to insert into
	bucketIdx := rt.bucketIndex(new.ID)
	insertBucket := rt.Buckets[bucketIdx]

	if pHeapIdx, ok := insertBucket.positions[*new.ID]; ok {
		// node is already in the bucket, so update it and re-heap
		insertBucket.activePeers[pHeapIdx] = new
		heap.Fix(insertBucket, pHeapIdx)
		return Existed
	}

	if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		heap.Push(insertBucket, new)
		rt.Peers[*new.ID] = new
		return Added
	}

	if insertBucket.containsSelf {
		// no vacancy in the bucket and it contains the self ID, so split the bucket and
		// insert via (single) recursive call
		rt.splitBucket(bucketIdx)
		rt.Push(new)
	}

	// no vacancy in the bucket and it doesn't contain the self ID, so just drop new peer on
	// the floor
	return Dropped
}

// Pop removes and returns the k peers in the bucket(s) closest to the given target.
func (rt *RoutingTable) Pop(target *big.Int, k uint) []*peer.Peer {
	if np := rt.NumPeers(); k > np {
		// if we're requesting more peers than we have, just return number we have
		k = np
	}

	fwdIdx := rt.bucketIndex(target)
	bkwdIdx := fwdIdx - 1

	// loop until we've populated all the peers or we have no more buckets to draw from
	next := make([]*peer.Peer, k)
	for i := 0; i < k; {
		bucketIdx := rt.chooseBucketIndex(target, fwdIdx, bkwdIdx)

		// fill peers from this bucket
		for ; i < k && rt.Buckets[bucketIdx].Len() > 0; i++ {
			next[i] = heap.Pop(rt.Buckets[bucketIdx]).(*peer.Peer)
			delete(rt.Peers, *next[i].ID)
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
func (rt *RoutingTable) Peak(target *big.Int, k int) []*peer.Peer {
	popped := rt.Pop(target, k)

	// add the peers back
	for _, p := range popped {
		rt.Push(p)
	}
	return popped
}

// Len returns the current number of buckets in the routing table.
func (rt *RoutingTable) Len() int {
	return len(rt.Buckets)
}

// Less returns whether bucket i's ID lower bound is less than bucket j's.
func (rt *RoutingTable) Less(i, j int) bool {
	return rt.Buckets[i].lowerBound.Cmp(rt.Buckets[j].lowerBound) < 0
}

// Swap swaps buckets i and j.
func (rt *RoutingTable) Swap(i, j int) {
	rt.Buckets[i], rt.Buckets[j] = rt.Buckets[j], rt.Buckets[i]
}

// chooseBucketIndex returns either the forward or backward bucket index from which to draw peers
// for a target.
func (rt *RoutingTable) chooseBucketIndex(target *big.Int, fwdIdx int, bkwdIdx int) int {
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
		fwdDist := big.NewInt(0).Xor(target, rt.Buckets[fwdIdx].upperBound)
		bkwdDist := big.NewInt(0).Xor(target, rt.Buckets[bkwdIdx].lowerBound)

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
func (rt *RoutingTable) bucketIndex(target *big.Int) int {
	return sort.Search(len(rt.Buckets), func(i int) bool {
		return target.Cmp(rt.Buckets[i].upperBound) < 0
	})
}

// splitBucket splits the bucketIdx into two and relocates the nodes appropriately
func (rt *RoutingTable) splitBucket(bucketIdx int) {
	current := rt.Buckets[bucketIdx]

	// define the bounds of the two new buckets from those of the current bucket
	middle := splitLowerBound(current.lowerBound, current.depth)

	// create the new buckets
	left := &routingBucket{
		depth:          current.depth + 1,
		lowerBound:     current.lowerBound,
		upperBound:     middle,
		maxActivePeers: current.maxActivePeers,
		activePeers:    make([]*peer.Peer, 0),
		positions:      make(map[string]int),
	}
	left.containsSelf = left.Contains(rt.SelfID)

	right := &routingBucket{
		depth:          current.depth + 1,
		lowerBound:     middle,
		upperBound:     current.upperBound,
		maxActivePeers: current.maxActivePeers,
		activePeers:    make([]*peer.Peer, 0),
		positions:      make(map[string]int),
	}
	right.containsSelf = right.Contains(rt.SelfID)

	// fill the buckets with existing peers
	for _, p := range current.activePeers {
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

// RoutingBucket is collection of peers stored as a heap.
type routingBucket struct {
	// bit depth of the bucket in the routing table/tree (i.e., the length of the bit prefix).
	depth uint

	// (inclusive) lower bound of IDs in this bucket
	lowerBound *big.Int

	// (exclusive) upper bound of IDs in this bucket
	upperBound *big.Int

	// maximum number of active peers for the bucket.
	maxActivePeers int

	// active peers in the bucket.
	activePeers []*peer.Peer

	// positions (i.e., indices) of each peer (keyed by ID string) in the heap.
	positions map[string]int

	// whether the bucket contains the current node's ID.
	containsSelf bool
}

// newFirstBucket creates a new instance of the first bucket (spanning the entire ID range)
func newFirstBucket() *routingBucket {
	return &routingBucket{
		depth:          0,
		lowerBound:     id.LowerBound,
		upperBound:     id.UpperBound,
		maxActivePeers: defaultMaxActivePeers,
		activePeers:    make([]*peer.Peer, 0),
		positions:      make(map[string]int),
		containsSelf:   true,
	}
}

func (rb *routingBucket) Len() int {
	return len(rb.activePeers)
}

func (rb *routingBucket) Less(i, j int) bool {
	return rb.activePeers[i].Before(rb.activePeers[j])
}

func (rb *routingBucket) Swap(i, j int) {
	rb.activePeers[i], rb.activePeers[j] = rb.activePeers[j], rb.activePeers[i]
	rb.positions[*rb.activePeers[i].ID] = i
	rb.positions[*rb.activePeers[j].ID] = j
}

// Push adds a peer to the routing bucket.
func (rb *routingBucket) Push(p interface{}) {
	rb.activePeers = append(rb.activePeers, p.(*peer.Peer))
	rb.positions[*p.(*peer.Peer).ID] = len(rb.activePeers) - 1
}

// Pop removes the root peer from the routing bucket.
func (rb *routingBucket) Pop() interface{} {
	root := rb.activePeers[len(rb.activePeers)-1]
	rb.activePeers = rb.activePeers[0 : len(rb.activePeers)-1]
	delete(rb.positions, *root.ID)
	return root
}

// Vacancy returns whether the bucket has room for more peers.
func (rb *routingBucket) Vacancy() bool {
	return len(rb.activePeers) < rb.maxActivePeers
}

// Contains returns whether the bucket's ID range contains the target.
func (rb *routingBucket) Contains(target *big.Int) bool {
	return target.Cmp(rb.lowerBound) >= 0 && target.Cmp(rb.upperBound) < 0
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
