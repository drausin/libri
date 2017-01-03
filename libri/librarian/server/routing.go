package server

import (
	"container/heap"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/drausin/libri/libri/db"
	"github.com/gogo/protobuf/proto"
)

var (
	defaultMaxActivePeers = 20
	routingTableKey       = []byte("RoutingTableKey")
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

// Push adds a peer to the routing bucket.
func (rb *RoutingBucket) Push(p interface{}) {
	rb.ActivePeers = append(rb.ActivePeers, p.(*Peer))
	rb.Positions[p.(*Peer).IDStr] = len(rb.ActivePeers) - 1
}

// Pop removes the root peer from the routing bucket.
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

// Contains returns whether the bucket's ID range contains the target.
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

// NewEmptyRoutingTable creates a new routing table without peers.
func NewEmptyRoutingTable(selfID *big.Int) *RoutingTable {
	firstBucket := newFirstBucket()
	return &RoutingTable{
		SelfID:  selfID,
		Peers:   make(map[string]*Peer),
		Buckets: []*RoutingBucket{firstBucket},
	}
}

// NewRoutingTableWithPeers creates a new routing table from a collection of peers.
func NewRoutingTableWithPeers(selfID *big.Int, peers map[string]*Peer) (*RoutingTable, int, error) {
	rt := NewEmptyRoutingTable(selfID)
	nAdded := 0
	for _, peer := range peers {
		bucketIdx, existed, err := rt.AddPeer(peer)
		if err != nil {
			return nil, 0, err
		}
		if bucketIdx != -1 && !existed {
			// increment added peers it didn't already exist in the bucket
			nAdded++
		}
	}
	return rt, nAdded, nil
}

// NewRoutingTableFromStorage returns a new RoutingTable instance from a
// StoredRoutingTable instance.
func NewRoutingTableFromStorage(stored *StoredRoutingTable) (*RoutingTable, error) {
	peers := make(map[string]*Peer)
	var err error
	for idStr, storedPeer := range stored.Peers {
		peers[idStr], err = NewPeerFromStorage(storedPeer)
		if err != nil {
			return nil, err
		}
	}
	selfID := big.NewInt(0).SetBytes(stored.SelfId)
	rt, _, err := NewRoutingTableWithPeers(selfID, peers)
	return rt, err
}

// Load retrieves the routing table form the KV DB.
func LoadRoutingTable(db db.KVDB) (*RoutingTable, error) {
	storedRoutingTableB, err := db.Get(routingTableKey)
	if storedRoutingTableB == nil || err != nil {
		return nil, err
	}
	stored := &StoredRoutingTable{}
	err = proto.Unmarshal(storedRoutingTableB, stored)
	if err != nil {
		return nil, err
	}
	return NewRoutingTableFromStorage(stored)
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

// NewStoredRoutingTable creates a new StoredRoutingTable instance from the RoutingTable instance.
func (rt *RoutingTable) NewStoredRoutingTable() *StoredRoutingTable {
	storedPeers := make(map[string]*StoredPeer, len(rt.Peers))
	for idStr, peer := range rt.Peers {
		storedPeers[idStr] = peer.NewStoredPeer()
	}
	return &StoredRoutingTable{
		SelfId: rt.SelfID.Bytes(),
		Peers:  storedPeers,
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

// Save stores a representation of the routing table to the KV DB.
func (rt *RoutingTable) Save(db db.KVDB) error {
	storedRoutingTableB, err := proto.Marshal(rt.NewStoredRoutingTable())
	if err != nil {
		return err
	}
	return db.Put(routingTableKey, storedRoutingTableB)
}

// Close disconnects all client connections and saves the routing table state to the DB.
func (rt *RoutingTable) Close(db db.KVDB) error {
	// disconnect from all peers
	for _, peer := range rt.Peers {
		if err := peer.Disconnect(); err != nil {
			return err
		}
	}

	// save to DB
	return rt.Save(db)
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
			return bucketIdx, true, fmt.Errorf("existing peer does not have same "+
				"nodeId (%s) as new peer (%s)", existing.ID, new.ID)
		}
		if _, exists := rt.Peers[new.IDStr]; !exists {
			return bucketIdx, true, fmt.Errorf("existing peer (%v) not found in "+
				"routing table peers map", new.IDStr)
		}

		// move node to bottom of heap
		heap.Remove(insertBucket, pHeapIdx)
		heap.Push(insertBucket, new)

		return bucketIdx, true, nil
	}

	if insertBucket.Vacancy() {
		// node isn't already in the bucket and there's vacancy, so add it
		if _, exists := rt.Peers[new.IDStr]; exists {
			return bucketIdx, true, fmt.Errorf("new peer (%v) should not be found in "+
				"routing table peers map", new.IDStr)
		}

		heap.Push(insertBucket, new)
		rt.Peers[new.IDStr] = new

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
		return nil, nil, fmt.Errorf("number of peers (n = %d) must be positive", k)
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
			next[i] = heap.Pop(rt.Buckets[bucketIdx]).(*Peer)
			bucketIdxs[i] = bucketIdx
			delete(rt.Peers, next[i].IDStr)
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

// PeakNextPeers returns (but does not remove) the n peers in the bucket(s) closest to the given
// target.
func (rt *RoutingTable) PeakNextPeers(target *big.Int, k int) ([]*Peer, []int, error) {
	nextPeers, bucketIdxs, err := rt.PopNextPeers(target, k)
	if err != nil {
		return nil, nil, err
	}

	// add the peers back into their respective buckets
	for i, nextPeer := range nextPeers {
		if _, exists := rt.Peers[nextPeer.IDStr]; exists {
			return nil, nil, fmt.Errorf("new peer (%v) should not be found in "+
				"routing table peers map", nextPeer.IDStr)
		}
		rt.Buckets[bucketIdxs[i]].Push(nextPeer)
		rt.Peers[nextPeer.IDStr] = nextPeer
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
	return big.NewInt(0).SetBit(lowerBound, int(IDLength*8-depth-1), 1)
}
