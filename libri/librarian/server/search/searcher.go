package search

import (
	"bytes"
	"container/heap"
	"fmt"
	"sync"

	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/gogo/protobuf/proto"
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
	// signs queries
	signer signature.Signer

	// issues find queries to the peers
	querier Querier

	// processes the find query responses from the peers
	rp FindResponseProcessor
}

// NewSearcher returns a new Searcher with the given Querier and ResponseProcessor.
func NewSearcher(s signature.Signer, q Querier, rp FindResponseProcessor) Searcher {
	return &searcher{signer: s, querier: q, rp: rp}
}

// NewDefaultSearcher creates a new Searcher with default sub-object instantiations.
func NewDefaultSearcher(peerID ecid.ID) Searcher {
	return NewSearcher(
		signature.NewSigner(peerID.Key()),
		NewQuerier(),
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
	ctx, cancel, err := NewSignedTimeoutContext(s.signer, search.Request, search.Params.Timeout)
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

// NewSignedTimeoutContext creates a new context with a timeout and request signature.
func NewSignedTimeoutContext(signer signature.Signer, request proto.Message,
	timeout time.Duration) (context.Context, context.CancelFunc, error) {
	ctx := context.Background()

	// sign the message
	signedJWT, err := signer.Sign(request)
	if err != nil {
		return nil, func() {}, err
	}
	ctx = context.WithValue(ctx, signature.NewContextKey(), signedJWT)

	// add timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)

	return ctx, cancel, nil
}

// Querier handles Find queries to a peer.
type Querier interface {
	// Query uses a peer connection to query for a particular key with an api.FindRequest and
	// returns its response.
	Query(ctx context.Context, pConn peer.Connector, rq *api.FindRequest,
		opts ...grpc.CallOption) (*api.FindResponse, error)
}

type querier struct{}

// NewQuerier creates a new FindQuerier instance for FindPeers queries.
func NewQuerier() Querier {
	return &querier{}
}

func (q *querier) Query(ctx context.Context, pConn peer.Connector, rq *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Find(ctx, rq, opts...)
}

// FindResponseProcessor handles an api.FindResponse
type FindResponseProcessor interface {
	// Process handles an api.FindResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap.
	Process(*api.FindResponse, *Result) error
}

type findResponseProcessor struct {
	peerFromer peer.Fromer
}

// NewResponseProcessor creates a new ResponseProcessor instance.
func NewResponseProcessor(peerFromer peer.Fromer) FindResponseProcessor {
	return &findResponseProcessor{peerFromer: peerFromer}
}

// Process processes an api.FindResponse, updating the result with the newly found peers.
func (frp *findResponseProcessor) Process(rp *api.FindResponse, result *Result) error {
	if rp.Value != nil {
		// response has value we're searching for
		result.Value = rp.Value
		return nil
	}

	if rp.Addresses != nil {
		// response has peer addresses close to key
		for _, pa := range rp.Addresses {
			newID := cid.FromBytes(pa.PeerId)
			if !result.Closest.In(newID) && !result.Unqueried.In(newID) {
				// only add discovered peers that we haven't already seen
				newPeer := frp.peerFromer.FromAPI(pa)
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
