package search

import (
	"bytes"
	"container/heap"
	"fmt"
	"sync"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/pkg/errors"
)

// Searcher executes searches for particular keys.
type Searcher interface {
	// Search executes a search from a list of seeds.
	Search(search *Search, seeds []peer.Peer) error
}

type searcher struct {
	// signs queries
	signer signature.Signer

	// issues find queries to the peers
	querier client.FindQuerier

	// processes the find query responses from the peers
	rp ResponseProcessor
}

// NewSearcher returns a new Searcher with the given Querier and ResponseProcessor.
func NewSearcher(s signature.Signer, q client.FindQuerier, rp ResponseProcessor) Searcher {
	return &searcher{signer: s, querier: q, rp: rp}
}

// NewDefaultSearcher creates a new Searcher with default sub-object instantiations.
func NewDefaultSearcher(signer signature.Signer) Searcher {
	return NewSearcher(
		signer,
		client.NewFindQuerier(),
		NewResponseProcessor(peer.NewFromer()),
	)
}

func (s *searcher) Search(search *Search, seeds []peer.Peer) error {
	if err := search.Result.Unqueried.SafePushMany(seeds); err != nil {
		search.Result.FatalErr = err
		return search.Result.FatalErr
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
		search.mu.Unlock()
		if _, err := next.Connector().Connect(); err != nil {
			// if we have issues connecting, skip to next peer
			continue
		}

		// do the query
		response, err := s.query(next.Connector(), search)
		if err != nil {
			// if we had an issue querying, skip to next peer
			search.mu.Lock()
			search.Result.NErrors++
			next.Recorder().Record(peer.Response, peer.Error)
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
			search.mu.Lock()
			search.Result.FatalErr = err
			search.mu.Unlock()
			return
		}

		// add next peer to set of peers that responded
		search.mu.Lock()
		if _, in := search.Result.Responded[next.ID().String()]; !in {
			search.Result.Responded[next.ID().String()] = next
		}
		search.mu.Unlock()
	}
}

func (s *searcher) query(pConn peer.Connector, search *Search) (*api.FindResponse, error) {
	ctx, cancel, err := client.NewSignedTimeoutContext(s.signer, search.Request,
		search.Params.Timeout)
	defer cancel()
	if err != nil {
		return nil, err
	}

	rp, err := s.querier.Query(ctx, pConn, search.Request)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, search.Request.Metadata.RequestId) {
		return nil, fmt.Errorf("unexpected response request ID received: %v, "+
			"expected %v", rp.Metadata.RequestId, search.Request.Metadata.RequestId)
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
			newID := cid.FromBytes(pa.PeerId)
			if !result.Closest.In(newID) && !result.Unqueried.In(newID) {
				// only add discovered peers that we haven't already seen
				newPeer := frp.fromer.FromAPI(pa)
				if err := result.Unqueried.SafePush(newPeer); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// invalid response
	return errors.New("FindResponse contains neither value nor peer addresses")
}
