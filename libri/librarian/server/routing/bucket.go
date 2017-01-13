package routing

import (
	"math/big"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

// bucket is collection of peers stored as a heap.
type bucket struct {
	// bit depth of the bucket in the routing table/tree (i.e., the length of the bit prefix).
	Depth uint

	// (inclusive) lower bound of IDs in this bucket
	LowerBound *big.Int

	// (exclusive) upper bound of IDs in this bucket
	UpperBound *big.Int

	// whether the bucket contains the current node's ID.
	ContainsSelf bool

	// maximum number of active peers for the bucket.
	MaxActivePeers int

	// active peers in the bucket.
	ActivePeers []*peer.Peer

	// positions (i.e., indices) of each peer (keyed by ID string) in the heap.
	Positions map[string]int
}

// newFirstBucket creates a new instance of the first bucket (spanning the entire ID range)
func newFirstBucket() *bucket {
	return &bucket{
		Depth:          0,
		LowerBound:     id.LowerBound,
		UpperBound:     id.UpperBound,
		MaxActivePeers: defaultMaxActivePeers,
		ActivePeers:    make([]*peer.Peer, 0),
		Positions:      make(map[string]int),
		ContainsSelf:   true,
	}
}

func (b *bucket) Len() int {
	return len(b.ActivePeers)
}

func (b *bucket) Less(i, j int) bool {
	return b.ActivePeers[i].Before(b.ActivePeers[j])
}

func (b *bucket) Swap(i, j int) {
	b.ActivePeers[i], b.ActivePeers[j] = b.ActivePeers[j], b.ActivePeers[i]
	b.Positions[b.ActivePeers[i].IDStr] = i
	b.Positions[b.ActivePeers[j].IDStr] = j
}

// Push adds a peer to the routing bucket.
func (b *bucket) Push(p interface{}) {
	b.ActivePeers = append(b.ActivePeers, p.(*peer.Peer))
	b.Positions[p.(*peer.Peer).IDStr] = len(b.ActivePeers) - 1
}

// Pop removes the root peer from the routing bucket.
func (b *bucket) Pop() interface{} {
	root := b.ActivePeers[len(b.ActivePeers)-1]
	b.ActivePeers = b.ActivePeers[0 : len(b.ActivePeers)-1]
	delete(b.Positions, root.IDStr)
	return root
}

func (b *bucket) Before(c *bucket) bool {
	return b.LowerBound.Cmp(c.LowerBound) < 0
}

// Vacancy returns whether the bucket has room for more peers.
func (b *bucket) Vacancy() bool {
	return len(b.ActivePeers) < b.MaxActivePeers
}

// Contains returns whether the bucket's ID range contains the target.
func (b *bucket) Contains(target *big.Int) bool {
	return target.Cmp(b.LowerBound) >= 0 && target.Cmp(b.UpperBound) < 0
}
