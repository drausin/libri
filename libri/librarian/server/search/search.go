package search

import (
	"bytes"
	"container/heap"
	"fmt"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"golang.org/x/net/context"
	"time"
)

var (
	// maximum number of errors tolerated during a search
	DefaultNMaxErrors = uint(3)

	// number of parallel search workers
	DefaultConcurrency = uint(3)

	DefaultQueryTimeout = 5 * time.Second
)

// PeerSearcher searches for the peers closest to a given target.
type PeerSearcher interface {
	// Search executes a search from a list of seeds.
	Search(seeds []peer.Peer) (*SearchResult, error)
}

type SearchResult struct {
	target    cid.ID
	found     bool
	err       error
	closest   ClosestPeers
	responded []peer.Peer
}

type peerSearcher struct {
	// what we're searching for
	target cid.ID

	// required number of peers closest to the target we need to receive responses from
	nClosestResponses uint

	// maximum number of errors tolerated when querying peers during the search
	nMaxErrors uint

	// number of concurrent queries to use in search
	concurrency uint

	// queries the peers
	querier Querier

	// processes the responses from the peers
	responseProcessor ResponseProcessor
}

func NewSearcher(target cid.ID) *peerSearcher {
	return &peerSearcher{
		target:            target,
		nClosestResponses: routing.DefaultMaxActivePeers,
		nMaxErrors:        DefaultNMaxErrors,
		concurrency:       DefaultConcurrency,
		querier:           NewQuerier(target),
		responseProcessor: NewResponseProcessor(),
	}
}

func (s *peerSearcher) WithNClosestResponses(n uint) *peerSearcher {
	s.nClosestResponses = n
	return s
}

func (s *peerSearcher) WithQuerier(q Querier) *peerSearcher {
	s.querier = q
	return s
}

func (s *peerSearcher) WithResponseProcessor(rp ResponseProcessor) *peerSearcher {
	s.responseProcessor = rp
	return s
}

func (s *peerSearcher) Search(seeds []peer.Peer) (*SearchResult, error) {
	unqueried := newClosestPeers(s.target, s.concurrency)
	if err := unqueried.SafePushMany(seeds); err != nil {
		panic(err)
	}

	// max-heap of closest peers that have responded with the farther peer at the root
	closestResponded := newFarthestPeers(s.target, s.nClosestResponses)

	// list of all peers that responded successfully
	allResponded := make([]peer.Peer, 0)

	// main processing loop; prob should add timeout
	// TODO: parallelize this
	nErrors := uint(0)
	for !s.finished(unqueried, closestResponded) && nErrors < s.nMaxErrors &&
		unqueried.Len() > 0 {

		// get next peer to query
		next := heap.Pop(unqueried).(peer.Peer)
		nextClient, err := next.Connector().Connect()
		if err != nil {
			// if we have issues connecting, skip to next peer
			continue
		}

		// do the query
		response, err := s.querier.Query(nextClient)
		if err != nil {
			// if we had an issue querying, skip to next peer
			nErrors++
			next.Responses().Error()
			continue
		}
		next.Responses().Success()

		// add to heap of closest responded peers
		err = closestResponded.SafePush(next)
		if err != nil {
			return nil, err
		}

		// process the heap's response
		s.responseProcessor.Process(response, unqueried, closestResponded)

		// add next peer to list of peers that responded
		allResponded = append(allResponded, next)
	}

	if !s.finished(unqueried, closestResponded) {
		return &SearchResult{
			target: s.target,
			found:  false,
			err:    fmt.Errorf("encountered %d errors during find, giving up", nErrors),
		}, nil
	}

	return &SearchResult{
		target:    s.target,
		found:     true,
		closest:   closestResponded,
		responded: allResponded,
	}, nil
}

// finished returns whether the search is complete, which occurs when we have have received
// responses from the required number of peers, and the max distance of those peers to the target
// is less than the min distance of the peers we haven't queried yet.
func (s *peerSearcher) finished(unqueried ClosestPeers, responded FarthestPeers) bool {
	return uint(responded.Len()) == s.nClosestResponses &&
		responded.PeakDistance().Cmp(unqueried.PeakDistance()) < 0
}

// Querier handles querying a peer.
type Querier interface {
	// Query queries a Librarian client with an api.FindRequest and returns its response.
	Query(api.LibrarianClient) (*api.FindPeersResponse, error)
}

type querier struct {
	// what we're search for
	target cid.ID

	// number of peers to request
	nPeers uint

	// query timeout
	timeout time.Duration
}

func NewQuerier(target cid.ID) Querier {
	return &querier{
		target:  target,
		nPeers:  routing.DefaultMaxActivePeers,
		timeout: DefaultQueryTimeout,
	}
}

func (q *querier) Query(client api.LibrarianClient) (*api.FindPeersResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
	defer cancel()

	rq := api.NewFindRequest(q.target, q.nPeers)
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

// ResponseProcessor handles an api.FindPeersResponse
type ResponseProcessor interface {
	// Process handles an api.FindPeersResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap.
	Process(*api.FindPeersResponse, ClosestPeers, FarthestPeers) error
}

type responseProcessor struct {
	peerFromer peer.Fromer
}

func NewResponseProcessor() ResponseProcessor {
	return &responseProcessor{peerFromer: peer.NewFromer()}
}

func (rp *responseProcessor) Process(response *api.FindPeersResponse, unqueried ClosestPeers,
	closestResponded FarthestPeers) error {

	for _, pa := range response.Addresses {
		newID := cid.FromBytes(pa.PeerId)
		if !unqueried.In(newID) && !closestResponded.In(newID) {
			// only add discovered peers that we haven't already seen
			newPeer := rp.peerFromer.FromAPI(pa)
			if err := unqueried.SafePush(newPeer); err != nil {
				return err
			}
		}
	}
	return nil
}
