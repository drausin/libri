package routing

import (
	"math/big"

	"container/heap"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	gw "github.com/drausin/libri/libri/librarian/server/goodwill"
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
	judge gw.Judge
}

// newFirstBucket creates a new instance of the first bucket (spanning the entire ID range)
func newFirstBucket(maxActivePeers uint, judge gw.Judge) *bucket {
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
	// swap j & i b/c we want a max-heap, i.e., least-preferred peers at the top
	return b.judge.Prefer(b.activePeers[j].ID(), b.activePeers[i].ID())
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
	ps := make([]peer.Peer, 0, k)
	for _, p := range b.activePeers {
		if b.judge.Trust(p.ID(), api.Find, gw.Response) && b.judge.Healthy(p.ID()) {
			ps = append(ps, p)
			if len(ps) == int(k) {
				return ps
			}
		}
	}
	return ps
}

// Find finds the k healthy, good peers closest to the target.
func (b *bucket) Find(target id.ID, k uint) []peer.Peer {
	tp := newTargetedPeers(target, k)
	for _, p := range b.activePeers {
		if b.judge.Trust(p.ID(), api.Find, gw.Response) && b.judge.Healthy(p.ID()) {
			heap.Push(tp, p)
		}
		if uint(len(tp.peers)) > k {
			heap.Pop(tp)
		}
	}
	return tp.peers
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

type targetedPeers struct {
	target    id.ID
	distances []*big.Int
	peers     []peer.Peer
}

func newTargetedPeers(target id.ID, k uint) *targetedPeers {
	return &targetedPeers{
		target:    target,
		distances: make([]*big.Int, 0, k+1),
		peers:     make([]peer.Peer, 0, k+1),
	}
}

func (tp *targetedPeers) Len() int {
	return len(tp.peers)
}

func (tp *targetedPeers) Less(i, j int) bool {
	// swap j & i b/c we want a max heap
	return tp.distances[j].Cmp(tp.distances[i]) < 0
}

func (tp *targetedPeers) Pop() interface{} {
	root := tp.peers[len(tp.peers)-1]
	tp.distances = tp.distances[0 : len(tp.distances)-1]
	tp.peers = tp.peers[0 : len(tp.peers)-1]
	return root
}

func (tp *targetedPeers) Push(x interface{}) {
	p := x.(peer.Peer)
	dist := p.ID().Distance(tp.target)
	tp.distances = append(tp.distances, dist)
	tp.peers = append(tp.peers, p)
}

func (tp *targetedPeers) Swap(i, j int) {
	tp.peers[i], tp.peers[j] = tp.peers[j], tp.peers[i]
	tp.distances[i], tp.distances[j] = tp.distances[j], tp.distances[i]
}
