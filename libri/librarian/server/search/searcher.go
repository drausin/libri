package search

import (
	"bytes"
	"container/heap"
	"errors"
	"sync"
	"time"

	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

const searcherFindRetryTimeout = 25 * time.Millisecond

var (
	// ErrTooManyFindErrors indicates when a search has encountered too many Find request errors.
	ErrTooManyFindErrors = errors.New("too many Find errors")

	errInvalidResponse = errors.New("FindResponse contains neither value nor peer addresses")
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

type peerResponse struct {
	peer     peer.Peer
	response *api.FindResponse
	err      error
}

func (s *searcher) Search(search *Search, seeds []peer.Peer) error {
	err := search.Result.Unqueried.SafePushMany(seeds)
	cerrors.MaybePanic(err)

	toQuery := make(chan peer.Peer, search.Params.Concurrency)
	peerResponses := make(chan peerResponse, search.Params.Concurrency)

	nToQueryInitially := search.Params.Concurrency
	if nToQueryInitially > uint(search.Result.Unqueried.Len()) {
		nToQueryInitially = uint(search.Result.Unqueried.Len())
	}
	for c := uint(0); c < nToQueryInitially; c++ {
		search.wrapLock(func() {
			toQuery <- heap.Pop(search.Result.Unqueried).(peer.Peer)
		})
	}

	// goroutine that queues up next peer to query from unqueried heap
	go func() {
		for peerResponse := range peerResponses {
			if peerResponse.err != nil {
				recordError(peerResponse.peer, peerResponse.err, search)
			} else if err := s.rp.Process(peerResponse.response, search); err != nil {
				recordError(peerResponse.peer, err, search)
			} else {
				recordSuccess(peerResponse.peer, search)
			}
			if search.Finished() {
				break
			}

			search.mu.Lock()
			next := heap.Pop(search.Result.Unqueried).(peer.Peer)
			_, alreadyQueried := search.Result.Queried[next.ID().String()]
			search.mu.Unlock()
			if alreadyQueried {
				continue
			}
			search.wrapLock(func() {
				select {
				case <-toQuery:
					// already closed
					break
				case toQuery <- next:
				}
			})
			if search.Finished() {
				break
			}
		}
		search.wrapLock(func() { maybeClose(toQuery) })
	}()

	var wg1 sync.WaitGroup
	for c := uint(0); c < search.Params.Concurrency; c++ {
		wg1.Add(1)
		go func(wg2 *sync.WaitGroup) {
			defer wg2.Done()
			for next := range toQuery {
				if search.Finished() {
					break
				}
				search.AddQueried(next)
				response, err := s.query(next, search)
				peerResponses <- peerResponse{
					peer:     next,
					response: response,
					err:      err,
				}
			}
		}(&wg1)
	}

	wg1.Wait()
	return search.Result.FatalErr
}

func (s *searcher) query(p peer.Peer, search *Search) (*api.FindResponse, error) {
	findClient, err := s.finderCreator.Create(p.Connector())
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

func maybeClose(toQuery chan peer.Peer) {
	select {
	case <-toQuery:
	default:
		close(toQuery)
	}
}

func recordError(p peer.Peer, err error, s *Search) {
	s.wrapLock(func() {
		s.Result.Errored[p.ID().String()] = err
		if s.Errored() {
			s.Result.FatalErr = ErrTooManyFindErrors
		}
	})
	p.Recorder().Record(peer.Response, peer.Error)
}

func recordSuccess(p peer.Peer, s *Search) {
	s.wrapLock(func() {
		err := s.Result.Closest.SafePush(p)
		cerrors.MaybePanic(err) // should never happen
		s.Result.Responded[p.ID().String()] = p
	})
	p.Recorder().Record(peer.Response, peer.Success)
}

// ResponseProcessor handles an api.FindResponse
type ResponseProcessor interface {
	// Process handles an api.FindResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap.
	Process(*api.FindResponse, *Search) error
}

type responseProcessor struct {
	fromer peer.Fromer
}

// NewResponseProcessor creates a new ResponseProcessor instance.
func NewResponseProcessor(f peer.Fromer) ResponseProcessor {
	return &responseProcessor{fromer: f}
}

// Process processes an api.FindResponse, updating the result with the newly found peers.
func (frp *responseProcessor) Process(rp *api.FindResponse, search *Search) error {
	if rp.Value != nil {
		// response has value we're searching for
		search.Result.Value = rp.Value
		return nil
	}

	if rp.Peers != nil {
		// response has peer addresses close to key
		search.wrapLock(func() {
			AddPeers(search.Result.Queried, search.Result.Unqueried, rp.Peers, frp.fromer)
		})
		return nil
	}

	// invalid response
	return errInvalidResponse
}

// AddPeers adds a list of peer address to the unqueried heap.
func AddPeers(
	queried map[string]struct{},
	unqueried ClosestPeers,
	peers []*api.PeerAddress,
	fromer peer.Fromer,
) {
	for _, pa := range peers {
		newID := id.FromBytes(pa.PeerId)
		inUnqueried := unqueried.In(newID)
		_, inQueried := queried[newID.String()]
		if !inUnqueried && !inQueried {
			// only add discovered peers that we haven't already seen
			newPeer := fromer.FromAPI(pa)
			err := unqueried.SafePush(newPeer)
			cerrors.MaybePanic(err) // should never happen
		}
	}
}
