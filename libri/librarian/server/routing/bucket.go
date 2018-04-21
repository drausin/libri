package routing

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/goodwill"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

// bucket is collection of peers stored as a heap.
type bucket struct {
	// bit depth of the bucket in the routing table/tree (i.e., the length of the bit prefix).
	depth uint

	// (inclusive) lower bound of IDs in this bucket
	lowerBound id.ID

	// (exclusive) upper bound of IDs in this bucket
	upperBound id.ID

	// proportion of the 256-bit ID space that this bucket spans
	idMass float64

	// cumulative proportion of ID space spanned by this and all lower buckets
	idCumMass float64

	// whether the bucket contains the current node's ID.
	containsSelf bool

	// maximum number of active peers for the bucket.
	maxActivePeers uint

	// active peers in the bucket.
	activePeers []peer.Peer

	// positions (i.e., indices) of each peer (keyed by ID string) in the heap.
	positions map[string]int

	// determines peer ordering within a bucket
	judge goodwill.PreferJudge
}

// newFirstBucket creates a new instance of the first bucket (spanning the entire ID range)
func newFirstBucket(maxActivePeers uint, judge goodwill.PreferJudge) *bucket {
	return &bucket{
		depth:          0,
		lowerBound:     id.LowerBound,
		upperBound:     id.UpperBound,
		idMass:         1.0,
		idCumMass:      1.0,
		maxActivePeers: maxActivePeers,
		activePeers:    make([]peer.Peer, 0),
		positions:      make(map[string]int),
		containsSelf:   true,
		judge:          judge,
	}
}

func (b *bucket) Len() int {
	return len(b.activePeers)
}

func (b *bucket) Less(i, j int) bool {
	return b.judge.Prefer(b.activePeers[i].ID(), b.activePeers[j].ID())
}

func (b *bucket) Swap(i, j int) {
	b.activePeers[i], b.activePeers[j] = b.activePeers[j], b.activePeers[i]
	b.positions[b.activePeers[i].ID().String()] = i
	b.positions[b.activePeers[j].ID().String()] = j
}

// Push adds a peer to the routing bucket.
func (b *bucket) Push(p interface{}) {
	b.activePeers = append(b.activePeers, p.(peer.Peer))
	b.positions[p.(peer.Peer).ID().String()] = len(b.activePeers) - 1
}

// Pop removes the root peer from the routing bucket.
func (b *bucket) Pop() interface{} {
	root := b.activePeers[len(b.activePeers)-1]
	b.activePeers = b.activePeers[0 : len(b.activePeers)-1]
	delete(b.positions, root.ID().String())
	return root
}

func (b *bucket) Peak(k uint) []peer.Peer {
	if k >= uint(b.Len()) {
		return b.activePeers
	}
	return b.activePeers[:k]
}

func (b *bucket) Before(c *bucket) bool {
	return b.lowerBound.Cmp(c.lowerBound) < 0
}

// Vacancy returns whether the bucket has room for more peers.
func (b *bucket) Vacancy() bool {
	return len(b.activePeers) < int(b.maxActivePeers)
}

// Contains returns whether the bucket's ID range contains the target.
func (b *bucket) Contains(target id.ID) bool {
	return target.Cmp(b.lowerBound) >= 0 && target.Cmp(b.upperBound) < 0
}
