package search

import (
	"bytes"
	"fmt"
	"sync"

	"container/heap"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Searcher executes searches for particular targets.
type Searcher interface {
	// Search executes a search from a list of seeds.
	Search(search *Search, seeds []peer.Peer) error
}

type searcher struct {
	// queries the peers
	q Querier

	// processes the responses from the peers
	rp ResponseProcessor
}

// NewSearcher returns a new Searcher with the given Querier and ResponseProcessor.
func NewSearcher(q Querier, rp ResponseProcessor) Searcher {
	return &searcher{q: q, rp: rp}
}

func (s *searcher) Search(search *Search, seeds []peer.Peer) error {
	if err := search.result.unqueried.SafePushMany(seeds); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go s.do(search, &wg)
	wg.Wait()

	return search.fatalErr
}

func (s *searcher) do(search *Search, wg *sync.WaitGroup) {
	defer wg.Done()
	for !search.Finished() {

		// get next peer to query
		search.mu.Lock()
		next := heap.Pop(search.result.unqueried).(peer.Peer)
		search.mu.Unlock()
		nextClient, err := next.Connector().Connect()
		if err != nil {
			// if we have issues connecting, skip to next peer
			continue
		}

		// do the query
		response, err := s.q.Query(nextClient, search.target)
		if err != nil {
			// if we had an issue querying, skip to next peer
			search.mu.Lock()
			search.nErrors++
			search.mu.Unlock()
			next.Responses().Error()
			continue
		}
		next.Responses().Success()

		// add to heap of closest responded peers
		search.mu.Lock()
		err = search.result.closest.SafePush(next)
		search.mu.Unlock()
		if err != nil {
			search.fatalErr = err
			return
		}

		// process the heap's response
		search.mu.Lock()
		err = s.rp.Process(response, search.result)
		search.mu.Unlock()
		if err != nil {
			search.fatalErr = err
			return
		}

		// add next peer to set of peers that responded
		search.mu.Lock()
		if _, in := search.result.responded[next.ID().String()]; !in {
			search.result.responded[next.ID().String()] = next
		}
		search.mu.Unlock()
	}
}

// Querier handles Find queries to a peer.
type Querier interface {
	// Query queries a Librarian client for a particular target with an api.FindRequest and
	// returns its response.
	Query(api.LibrarianClient, cid.ID) (*api.FindResponse, error)
}

type querier struct {
	params *Parameters
	findFn func(client api.LibrarianClient, ctx context.Context, in *api.FindRequest,
		opts ...grpc.CallOption) (*api.FindResponse, error)
}

// NewFindPeersQuerier creates a new Querier instance for FindPeers queries.
func NewFindPeersQuerier(params *Parameters) Querier {
	return &querier{
		params: params,
		findFn: func(client api.LibrarianClient, ctx context.Context, in *api.FindRequest,
			opts ...grpc.CallOption) (*api.FindResponse, error) {
			return client.FindPeers(ctx, in, opts...)
		},
	}
}

// Query issues a Find query to the client for the given target.
func (q *querier) Query(client api.LibrarianClient, target cid.ID) (*api.FindResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), q.params.queryTimeout)
	defer cancel()

	rq := api.NewFindRequest(target, q.params.nClosestResponses)
	rp, err := q.findFn(client, ctx, rq)
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
	Process(*api.FindResponse, *Result) error
}

type findPeersResponseProcessor struct {
	peerFromer peer.Fromer
}

// NewFindPeersResponseProcessor creates a new ResponseProcessor instance.
func NewFindPeersResponseProcessor() ResponseProcessor {
	return &findPeersResponseProcessor{peerFromer: peer.NewFromer()}
}

// Process processes an api.FindResponse, updating the result with the newly found peers.
func (rp *findPeersResponseProcessor) Process(response *api.FindResponse, result *Result) error {
	for _, pa := range response.Addresses {
		newID := cid.FromBytes(pa.PeerId)
		if !result.closest.In(newID) && !result.unqueried.In(newID) {
			// only add discovered peers that we haven't already seen
			newPeer := rp.peerFromer.FromAPI(pa)
			if err := result.unqueried.SafePush(newPeer); err != nil {
				return err
			}
		}
	}
	return nil
}

// findValueResponseProcessor implement
type findValueResponseProcessor struct {
	fprp ResponseProcessor
}

// NewFindValueResponseProcessor creates a new ResponseProcessor instance.
func NewFindValueResponseProcessor() ResponseProcessor {
	return &findValueResponseProcessor{
		fprp: NewFindPeersResponseProcessor(),
	}
}

// Process adds the value or the found peers to the result.
func (rp *findValueResponseProcessor) Process(response *api.FindResponse, result *Result) error {
	if response.Value != nil {
		result.value = response.Value
	} else {
		if err := rp.fprp.Process(response, result); err != nil {
			return err
		}

	}
	return nil
}
