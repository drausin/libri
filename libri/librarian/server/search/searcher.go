package search

import (
	"bytes"
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

const searcherFindRetryTimeout = 100 * time.Millisecond

var (
	// ErrTooManyFindErrors indicates when a search has encountered too many Find request errors.
	ErrTooManyFindErrors = errors.New("too many Find errors")
)

// Searcher executes searches for particular keys.
type Searcher interface {
	// Search executes a search from a list of seeds.
	Search(search *Search, seeds []peer.Peer) error
}

type searcher struct {
	// signs queries
	signer client.Signer

	// creates api.Finders
	finderCreator client.FinderCreator

	// processes the find query responses from the peers
	rp ResponseProcessor
}

// NewSearcher returns a new Searcher with the given Querier and ResponseProcessor.
func NewSearcher(s client.Signer, c client.FinderCreator, rp ResponseProcessor) Searcher {
	return &searcher{signer: s, finderCreator: c, rp: rp}
}

// NewDefaultSearcher creates a new Searcher with default sub-object instantiations.
func NewDefaultSearcher(signer client.Signer) Searcher {
	return NewSearcher(
		signer,
		client.NewFinderCreator(),
		NewResponseProcessor(peer.NewFromer()),
	)
}

func (s *searcher) Search(search *Search, seeds []peer.Peer) error {
	if err := search.Result.Unqueried.SafePushMany(seeds); err != nil {
		panic(err) // should never happen
	}

	var wg sync.WaitGroup
	for c := uint(0); c < search.Params.Concurrency; c++ {
		wg.Add(1)
		go s.searchWork(search, &wg)
	}
	wg.Wait()

	return search.Result.FatalErr
}

func (s *searcher) searchWork(search *Search, wg *sync.WaitGroup) {
	defer wg.Done()
	for !search.Finished() {

		// get next peer to query
		search.mu.Lock()
		next := heap.Pop(search.Result.Unqueried).(peer.Peer)
		nextIDStr := next.ID().String()
		if _, in := search.Result.Responded[nextIDStr]; in {
			search.mu.Unlock()
			continue
		}
		if _, in := search.Result.Errored[nextIDStr]; in {
			search.mu.Unlock()
			continue
		}
		search.mu.Unlock()

		// do the query
		response, err := s.query(next.Connector(), search)
		if err != nil {
			// if we had an issue querying, skip to next peer
			search.mu.Lock()
			search.Result.Errored[nextIDStr] = err
			next.Recorder().Record(peer.Response, peer.Error)
			if search.Errored() {
				search.Result.FatalErr = ErrTooManyFindErrors
			}
			search.mu.Unlock()
			continue
		}
		search.mu.Lock()
		next.Recorder().Record(peer.Response, peer.Success)
		search.mu.Unlock()

		// process the heap's response
		search.mu.Lock()
		err = s.rp.Process(response, search.Result)
		search.mu.Unlock()
		if err != nil {
			search.mu.Lock()
			search.Result.FatalErr = err
			search.mu.Unlock()
			return
		}

		// add to heap of closest responded peers
		search.mu.Lock()
		err = search.Result.Closest.SafePush(next)
		search.mu.Unlock()
		if err != nil {
			panic(err) // should never happen
		}

		// add next peer to set of peers that responded
		search.mu.Lock()
		if _, in := search.Result.Responded[nextIDStr]; !in {
			search.Result.Responded[nextIDStr] = next
		}
		search.mu.Unlock()
	}
}

func (s *searcher) query(pConn api.Connector, search *Search) (*api.FindResponse, error) {
	findClient, err := s.finderCreator.Create(pConn)
	if err != nil {
		return nil, err
	}
	ctx, cancel, err := client.NewSignedTimeoutContext(s.signer, search.Request,
		search.Params.Timeout)
	if err != nil {
		return nil, err
	}
	retryFindClient := client.NewRetryFinder(findClient, searcherFindRetryTimeout)
	rp, err := retryFindClient.Find(ctx, search.Request)
	cancel()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, search.Request.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}

	return rp, nil
}

// ResponseProcessor handles an api.FindResponse
type ResponseProcessor interface {
	// Process handles an api.FindResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap.
	Process(*api.FindResponse, *Result) error
}

type responseProcessor struct {
	fromer peer.Fromer
}

// NewResponseProcessor creates a new ResponseProcessor instance.
func NewResponseProcessor(f peer.Fromer) ResponseProcessor {
	return &responseProcessor{fromer: f}
}

// Process processes an api.FindResponse, updating the result with the newly found peers.
func (frp *responseProcessor) Process(rp *api.FindResponse, result *Result) error {
	if rp.Value != nil {
		// response has value we're searching for
		result.Value = rp.Value
		return nil
	}

	if rp.Peers != nil {
		// response has peer addresses close to key
		for _, pa := range rp.Peers {
			newID := id.FromBytes(pa.PeerId)
			if !result.Closest.In(newID) && !result.Unqueried.In(newID) {
				// only add discovered peers that we haven't already seen
				newPeer := frp.fromer.FromAPI(pa)
				if err := result.Unqueried.SafePush(newPeer); err != nil {
					panic(err) // should never happen
				}
			}
		}
		return nil
	}

	// invalid response
	return errors.New("FindResponse contains neither value nor peer addresses")
}
