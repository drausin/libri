package search

import (
	"bytes"
	"container/heap"
	"fmt"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"golang.org/x/net/context"
	"math/big"
	"time"
)

// closePeers represents a heap of peers sorted by closest distance to a given target.
type closePeers struct {
	// target to compute distance to
	target *big.Int

	// peers we know about, sorted in a heap with farthest from the target at the root
	peers []peer.Peer

	// the distances of each peer to the target
	distances []*big.Int

	// set of IDStrs of the peers in the heap
	ids map[string]struct{}

	//  1: root is closest to target (min heap)
	// -1: root is farthest from target (max heap)
	sign int
}

func newClosePeersMinHeap(target *big.Int) *closePeers {
	return &closePeers{
		target:    target,
		peers:     make([]peer.Peer, 0),
		distances: make([]*big.Int, 0),
		ids:       make(map[string]struct{}),
		sign:      1,
	}
}

func newClosePeersMaxHeap(target *big.Int) *closePeers {
	return &closePeers{
		target:    target,
		peers:     make([]peer.Peer, 0),
		distances: make([]*big.Int, 0),
		ids:       make(map[string]struct{}),
		sign:      -1,
	}
}

// Len returns the current number of peers.
func (cp *closePeers) Len() int {
	return len(cp.peers)
}

// Less returns whether peer i is closer (or farther in case of max heap) to the target than peer j.
func (cp *closePeers) Less(i, j int) bool {
	return less(cp.sign, cp.distances[i], cp.distances[j])
}

func (cp *closePeers) distance(p peer.Peer) *big.Int {
	return id.Distance(p.ID(), cp.target)
}

func less(sign int, x, y *big.Int) bool {
	return sign*x.Cmp(y) < 0
}

// Swap swaps the peers in position i and j.
func (cp *closePeers) Swap(i, j int) {
	cp.peers[i], cp.peers[j] = cp.peers[j], cp.peers[i]
	cp.distances[i], cp.distances[j] = cp.distances[j], cp.distances[i]
}

func (cp *closePeers) Push(p interface{}) {
	cp.peers = append(cp.peers, p.(peer.Peer))
	cp.distances = append(cp.distances, cp.distance(p.(peer.Peer)))
	cp.ids[p.(peer.Peer).IDStr()] = struct{}{}
}

func (cp *closePeers) Pop() interface{} {
	root := cp.peers[len(cp.peers)-1]
	cp.peers = cp.peers[0 : len(cp.peers)-1]
	cp.distances = cp.distances[0 : len(cp.distances)-1]
	delete(cp.ids, root.IDStr())
	return root
}

func (cp *closePeers) In(p peer.Peer) bool {
	_, in := cp.ids[p.IDStr()]
	return in
}

// PeakDistance returns the distance from the root of the heap to the target.
func (cp *closePeers) PeakDistance() *big.Int {
	return cp.distances[0]
}

// PeakPeer returns (but does not remove) the the root of the heap.
func (cp *closePeers) PeakPeer() peer.Peer {
	return cp.peers[0]
}

type search struct {
	// what we're searching for
	target *big.Int

	// min heap of peers we know about but haven't yet queried
	unqueried *closePeers

	// max heap of peers we've sent FIND queries to and received responses from
	responded *closePeers

	// required number of peers closest to the target we need to receive responses from
	nClosestResponses uint
}

func newSearch(target *big.Int, nClosestResponses uint) *search {
	return &search{
		target:            target,
		unqueried:         newClosePeersMinHeap(target),
		responded:         newClosePeersMaxHeap(target),
		nClosestResponses: nClosestResponses,
	}
}

// finished returns whether the search is complete, which occurs when we have have received
// responses from the required number of peers, and the max distance of those peers to the target
// is less than the min distance of the peers we haven't queried yet.
func (s *search) finished() bool {
	return uint(s.responded.Len()) == s.nClosestResponses &&
		s.responded.PeakDistance().Cmp(s.unqueried.PeakDistance()) < 0
}

func (s *search) distance(p peer.Peer) *big.Int {
	return id.Distance(s.target, p.ID())
}

func (s *search) newFindRequest() *api.FindRequest {
	return api.NewFindRequest(s.target, s.nClosestResponses)
}

func (s *search) query(client api.LibrarianClient) (*api.FindPeersResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rq := s.newFindRequest()
	rp, err := client.FindPeers(ctx, rq)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.RequestId, rq.RequestId) {
		return nil, fmt.Errorf("unexpected response request ID received: %v, "+
			"expected %v", rp.RequestId, rq.RequestId)
	}

	return rp, nil
}

func (s *search) Closest() []peer.Peer {
	return s.responded.peers
}

func (s *search) Seen(p peer.Peer) bool {
	return s.responded.In(p) || s.unqueried.In(p)
}

type findResult struct {
	target  *big.Int
	found   bool
	err     error
	closest []peer.Peer
}

func Find(target *big.Int, rt routing.Table, nClosestResponses uint, concurrency uint) *findResult {
	s := newSearch(target, nClosestResponses)
	maxErrs := 3

	// get the initial set of peers to start search with
	for _, seed := range rt.Peak(target, concurrency) {
		heap.Push(s.unqueried, seed)
	}

	// main processing loop; prob should add timeout
	nErr := 0
	for !s.finished() && nErr < maxErrs {

		// get next peer to query
		next := heap.Pop(s.unqueried).(peer.Peer)

		client, err := next.MaybeConnect()
		if err != nil {
			continue
		}

		response, err := s.query(client)
		if err != nil {
			// increment error count if something went wrong
			next.RecordResponseError()
			nErr++
			continue
		}
		next.RecordResponseSuccess()

		// add or update peer in routing table
		rt.Push(next)

		// add peer to responded heap
		if s.responded.In(next) {
			// should never happen but check just in case
			panic(fmt.Errorf("peer unexpectedly already in responded heap: %v", next))

		}
		heap.Push(s.responded, next)

		if uint(s.responded.Len()) > s.nClosestResponses {
			// this will lower the upper bound on peer distance to target
			heap.Pop(s.responded)
		}

		for _, a := range response.Addresses {
			discovered := peer.FromAPIAddress(a)
			if !s.Seen(discovered) {
				// only add discovered peers that we haven't already seen
				heap.Push(s.unqueried, discovered)
			}
		}
	}

	if !s.finished() {
		return &findResult{
			target: target,
			found:  s.finished(),
			err:    fmt.Errorf("encountered %d errors during find, giving up", nErr),
		}
	}

	return &findResult{
		target:  target,
		found:   s.finished(),
		closest: s.Closest(),
	}
}
