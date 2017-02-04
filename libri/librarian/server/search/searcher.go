package search

import (
	"bytes"
	"fmt"
	"sync"

	"container/heap"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Searcher executes searches for particular keys.
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
	for c := uint(0); c < search.params.concurrency; c++ {
		wg.Add(1)
		go s.do(search, &wg)
	}
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
		response, err := s.q.Query(nextClient, search.key)
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
	// Query queries a Librarian client for a particular key with an api.FindRequest and
	// returns its response.
	Query(api.LibrarianClient, cid.ID) (*api.FindResponse, error)
}

type querier struct {
	params *Parameters
	finder Finder
}

// NewQuerier creates a new Querier instance for FindPeers queries.
func NewQuerier(params *Parameters, f Finder) Querier {
	return &querier{
		params: params,
		finder: f,
	}
}

// Query issues a Find query to the client for the given key.
func (q *querier) Query(client api.LibrarianClient, key cid.ID) (*api.FindResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), q.params.queryTimeout)
	defer cancel()

	rq := api.NewFindRequest(key, q.params.nClosestResponses)
	rp, err := q.finder.Find(ctx, client, rq)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.RequestId, rq.RequestId) {
		return nil, fmt.Errorf("unexpected response request ID received: %v, "+
			"expected %v", rp.RequestId, rq.RequestId)
	}

	return rp, nil
}

// Finder issues Find queries to api.LibrarianClients.
type Finder interface {
	// Find issues a Find query to an api.LibrarianClient.
	Find(ctx context.Context, client api.LibrarianClient, fr *api.FindRequest,
		opts ...grpc.CallOption) (*api.FindResponse, error)
}

// NewFinder creates a new Finder instance.
func NewFinder() Finder {
	return &finder{}
}

type finder struct{}

func (f *finder) Find(ctx context.Context, client api.LibrarianClient, fr *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	return client.Find(ctx, fr, opts...)
}

// ResponseProcessor handles an api.FindPeersResponse
type ResponseProcessor interface {
	// Process handles an api.FindPeersResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap.
	Process(*api.FindResponse, *Result) error
}

type responseProcessor struct {
	peerFromer peer.Fromer
}

// NewResponseProcessor creates a new ResponseProcessor instance.
func NewResponseProcessor(peerFromer peer.Fromer) ResponseProcessor {
	return &responseProcessor{peerFromer: peerFromer}
}

// Process processes an api.FindResponse, updating the result with the newly found peers.
func (rp *responseProcessor) Process(fr *api.FindResponse, result *Result) error {
	if fr.Value != nil {
		// response has value we're searching for
		result.value = fr.Value
		return nil
	}

	if fr.Addresses != nil {
		// response has peer addresses close to key
		for _, pa := range fr.Addresses {
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

	// invalid response
	return errors.New("FindResponse contains neither value nor peer addresses")
}
