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
	toQuery := make(chan peer.Peer, 1)
	peerResponses := make(chan *peerResponse, 1)

	// add seeds and queue some of them for querying
	err := search.Result.Unqueried.SafePushMany(seeds)
	cerrors.MaybePanic(err)

	go func() {
		for c := uint(0); c < search.Params.Concurrency; c++ {
			if next := getNextToQuery(search); next != nil {
				toQuery <- next
			}
		}
	}()

	// goroutine that processes responses and queues up next peer to query
	var wg1 sync.WaitGroup
	wg1.Add(1)
	go func(wg2 *sync.WaitGroup) {
		defer wg2.Done()
		toQueryClosed := false
		for pr := range peerResponses {
			processAnyReponse(pr, s.rp, search)
			if !toQueryClosed {
				toQueryClosed = maybeSendNextToQuery(toQuery, search)
			}
		}
	}(&wg1)

	// concurrent goroutines that issue queries to peers
	var wg3 sync.WaitGroup
	for c := uint(0); c < search.Params.Concurrency; c++ {
		wg3.Add(1)
		go func(wg4 *sync.WaitGroup) {
			defer wg4.Done()
			for next := range toQuery {
				search.AddQueried(next)
				response, err := s.query(next.Connector(), search)
				peerResponses <- &peerResponse{
					peer:     next,
					response: response,
					err:      err,
				}
			}
		}(&wg3)
	}
	wg3.Wait()
	close(peerResponses)

	wg1.Wait()
	return search.Result.FatalErr
}

func (s *searcher) query(pConn peer.Connector, search *Search) (*api.FindResponse, error) {
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

func getNextToQuery(search *Search) peer.Peer {
	if search.Finished() {
		return nil
	}
	search.mu.Lock()
	defer search.mu.Unlock()
	if search.Result.Unqueried.Len() == 0 {
		return nil
	}
	next := heap.Pop(search.Result.Unqueried).(peer.Peer)
	if _, alreadyQueried := search.Result.Queried[next.ID().String()]; alreadyQueried {
		return nil
	}
	return next
}

func maybeSendNextToQuery(toQuery chan peer.Peer, search *Search) bool {
	if stopQuerying := search.Finished() || search.Exhausted(); stopQuerying {
		close(toQuery)
		return true
	}
	if next := getNextToQuery(search); next != nil {
		toQuery <- next
	}
	return false
}

func processAnyReponse(pr *peerResponse, rp ResponseProcessor, search *Search) {
	if pr.err != nil {
		recordError(pr.peer, pr.err, search)
	} else if err := rp.Process(pr.response, search); err != nil {
		recordError(pr.peer, err, search)
	} else {
		recordSuccess(pr.peer, search)
	}
}

func recordError(p peer.Peer, err error, s *Search) {
	s.wrapLock(func() {
		s.Result.Errored[p.ID().String()] = err
	})
	if s.Errored() {
		s.wrapLock(func() {
			s.Result.FatalErr = ErrTooManyFindErrors
		})
	}
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
func (frp *responseProcessor) Process(rp *api.FindResponse, s *Search) error {
	if rp.Value != nil {
		// response has value we're searching for
		s.Result.Value = rp.Value
		return nil
	}

	if rp.Peers != nil {
		// response has peer addresses close to keys
		if !s.Finished() {
			// don't add peers to unqueried if the search is already finished and we're not going
			// to query those peers; there's a small chance that one of the peer that would be added
			// to unqueried would be closer than farthest closest peer, which would be then move the
			// search state back from finished -> not finished, a potentially confusing change
			// that we choose to avoid altogether at the expense of (very) occasionally missing a
			// closer peer
			s.wrapLock(func() {
				AddPeers(s.Result.Queried, s.Result.Unqueried, rp.Peers, frp.fromer)
			})
		}
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
