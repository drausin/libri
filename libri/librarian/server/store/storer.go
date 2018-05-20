package store

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
)

const storerStoreRetryTimeout = 25 * time.Millisecond

var (
	// ErrTooManyStoreErrors indicates when a store has encountered too many Store request errors.
	ErrTooManyStoreErrors = errors.New("too many Find errors")
)

// Storer executes store operations.
type Storer interface {
	// Store executes a store operation, starting with a given set of seed peers.
	Store(store *Store, seeds []peer.Peer) error
}

type storer struct {
	signer        client.Signer
	searcher      search.Searcher
	storerCreator client.StorerCreator
	rec           comm.QueryRecorder
}

// NewStorer creates a new Storer instance with given Searcher and StoreQuerier instances.
func NewStorer(
	signer client.Signer, rec comm.QueryRecorder, searcher search.Searcher, c client.StorerCreator,
) Storer {
	return &storer{
		signer:        signer,
		searcher:      searcher,
		storerCreator: c,
		rec:           rec,
	}
}

// NewDefaultStorer creates a new Storer with default Searcher and StoreQuerier instances.
func NewDefaultStorer(peerID ecid.ID, rec comm.QueryRecorder, clients client.Pool) Storer {
	signer := client.NewSigner(peerID.Key())
	return NewStorer(
		signer,
		rec,
		search.NewDefaultSearcher(signer, rec, clients),
		client.NewStorerCreator(clients),
	)
}

type peerResponse struct {
	peer     peer.Peer
	response *api.StoreResponse
	err      error
}

func (s *storer) Store(store *Store, seeds []peer.Peer) error {
	if len(seeds) < int(store.Params.Concurrency) {
		// fall back to single worker when we have insufficient seeds (usually only the case for
		// demo clusters with 3 or so peers)
		store.Params.Concurrency = 1
		store.Search.Params.Concurrency = 1
	}

	if !store.Search.Finished() {
		// sometimes the search has been pre-populated (e.g., by a verification) and is already
		// finished, so only do search if that's not the case
		if err := s.searcher.Search(store.Search, seeds); err != nil {
			store.Result = NewFatalResult(err)
			return err
		}
	}
	store.Result = NewInitialResult(store.Search.Result)

	// queue of peers to send Store requests to
	toQuery := make(chan peer.Peer, store.Params.NReplicas)
	peerResponses := make(chan *peerResponse, store.Params.NReplicas)

	for c := uint(0); c < store.Params.NReplicas; c++ {
		sendNextToQuery(toQuery, store)
	}

	var wg1 sync.WaitGroup
	wg1.Add(1)
	go func(wg2 *sync.WaitGroup) {
		defer wg2.Done()
		for peerResponse := range peerResponses {
			s.processAnyReponse(peerResponse, toQuery, store)
		}
	}(&wg1)

	var wg3 sync.WaitGroup
	for c := uint(0); c < store.Params.Concurrency; c++ {
		wg3.Add(1)
		go func(wg4 *sync.WaitGroup) {
			defer wg4.Done()
			for next := range toQuery {
				response, err := s.query(next, store)
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
	return store.Result.FatalErr
}

func (s *storer) query(next peer.Peer, store *Store) (*api.StoreResponse, error) {
	lc, err := s.storerCreator.Create(next.Address().String())
	if err != nil {
		return nil, err
	}
	ctx, cancel, err := client.NewSignedTimeoutContext(s.signer, store.Request,
		store.Params.Timeout)
	if err != nil {
		return nil, err
	}
	retryStoreClient := client.NewRetryStorer(lc, storerStoreRetryTimeout)
	rp, err := retryStoreClient.Store(ctx, store.Request)
	cancel()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, store.Request.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}

	return rp, nil
}

func (s *storer) processAnyReponse(pr *peerResponse, toQuery chan peer.Peer, store *Store) {
	errored := false
	if pr.err != nil {
		// if we had an issue querying, skip to next peer
		store.wrapLock(func() {
			store.Result.Errors = append(store.Result.Errors, pr.err)
		})
		if store.Errored() {
			store.wrapLock(func() {
				store.Result.FatalErr = ErrTooManyStoreErrors
			})
		}
		errored = true
		comm.MaybeRecordRpErr(s.rec, pr.peer.ID(), api.Store, pr.err)
	} else {
		store.wrapLock(func() {
			store.Result.Responded = append(store.Result.Responded, pr.peer)
		})
		s.rec.Record(pr.peer.ID(), api.Store, comm.Response, comm.Success)
	}

	finished := store.Finished()
	if errored && !finished {
		// since we've already queue NReplicas into toQuery above, only queue more
		// if we get an error
		sendNextToQuery(toQuery, store)
	} else if finished {
		maybeClose(toQuery)
	}
}

func maybeClose(toQuery chan peer.Peer) {
	select {
	case <-toQuery:
	default:
		close(toQuery)
	}
}

func getNextToQuery(store *Store) peer.Peer {
	if store.Finished() {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.Result.Unqueried) == 0 {
		return nil
	}
	next := store.Result.Unqueried[0]
	store.Result.Unqueried = store.Result.Unqueried[1:]
	return next
}

func sendNextToQuery(toQuery chan peer.Peer, store *Store) bool {
	if next := getNextToQuery(store); next != nil {
		toQuery <- next
	}
	return store.Finished()
}
