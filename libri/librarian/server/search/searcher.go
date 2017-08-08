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
	cerrors.MaybePanic(search.Result.Unqueried.SafePushMany(seeds))

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
	result := search.Result
	for !search.Finished() {

		// get next peer to query
		search.mu.Lock()
		next, nextIDStr := NextPeerToQuery(result.Unqueried, result.Responded, result.Errored)
		search.mu.Unlock()
		if next == nil {
			// next peer has already responded or errored
			continue
		}

		// do the query
		response, err := s.query(next.Connector(), search)
		if err != nil {
			// if we had an issue querying, skip to next peer
			search.wrapLock(func() { recordError(nextIDStr, next, err, search) })
			continue
		}

		// process the response
		search.wrapLock(func() { err = s.rp.Process(response, result) })
		if err != nil {
			search.wrapLock(func() { recordError(nextIDStr, next, err, search) })
			continue
		}

		// bookkeep ok response
		search.wrapLock(func() {
			next.Recorder().Record(peer.Response, peer.Success)
			cerrors.MaybePanic(result.Closest.SafePush(next))
			result.Responded[nextIDStr] = next
		})
	}
}

func recordError(nextIDStr string, p peer.Peer, err error, s *Search) {
	s.Result.Errored[nextIDStr] = err
	p.Recorder().Record(peer.Response, peer.Error)
	if s.Errored() {
		s.Result.FatalErr = ErrTooManyFindErrors
	}
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

// NextPeerToQuery pops the next peer from unqueried and returns it if it hasn't responded or
// errored yet. Otherwise it returns nil.
func NextPeerToQuery(
	unqueried heap.Interface, responded map[string]peer.Peer, errored map[string]error,
) (peer.Peer, string) {
	next := heap.Pop(unqueried).(peer.Peer)
	nextIDStr := next.ID().String()
	if _, in := responded[nextIDStr]; in {
		return nil, ""
	}
	if _, in := errored[nextIDStr]; in {
		return nil, ""
	}
	return next, nextIDStr
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
		AddPeers(result.Closest, result.Unqueried, rp.Peers, frp.fromer)
		return nil
	}

	// invalid response
	return errors.New("FindResponse contains neither value nor peer addresses")
}

// AddPeers adds a list of peer address to the unqueried heap.
func AddPeers(
	closest FarthestPeers, unqueried ClosestPeers, peers []*api.PeerAddress, fromer peer.Fromer,
) {
	for _, pa := range peers {
		newID := id.FromBytes(pa.PeerId)
		if !closest.In(newID) && !unqueried.In(newID) {
			// only add discovered peers that we haven't already seen
			newPeer := fromer.FromAPI(pa)
			if err := unqueried.SafePush(newPeer); err != nil {
				panic(err) // should never happen
			}
		}
	}
}
