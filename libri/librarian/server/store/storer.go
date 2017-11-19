package store

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
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
	// signs queries
	signer client.Signer

	// searcher is used for the first search half of the store operation
	searcher search.Searcher

	// issues store queries to the peers
	storerCreator client.StorerCreator
}

// NewStorer creates a new Storer instance with given Searcher and StoreQuerier instances.
func NewStorer(signer client.Signer, searcher search.Searcher, c client.StorerCreator) Storer {
	return &storer{
		signer:        signer,
		searcher:      searcher,
		storerCreator: c,
	}
}

// NewDefaultStorer creates a new Storer with default Searcher and StoreQuerier instances.
func NewDefaultStorer(peerID ecid.ID) Storer {
	signer := client.NewSigner(peerID.Key())
	return NewStorer(
		signer,
		search.NewDefaultSearcher(signer),
		client.NewStorerCreator(),
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

	if err := s.searcher.Search(store.Search, seeds); err != nil {
		store.Result = NewFatalResult(err)
		return err
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
			errored, finished := processAnyReponse(peerResponse, store)
			if errored && !finished {
				// since we've already queue NReplicas into toQuery above, only queue more
				// if we get an error
				sendNextToQuery(toQuery, store)
			} else if finished {
				maybeClose(toQuery)
			}
		}
	}(&wg1)

	var wg3 sync.WaitGroup
	for c := uint(0); c < store.Params.Concurrency; c++ {
		wg3.Add(1)
		go func(wg4 *sync.WaitGroup) {
			defer wg4.Done()
			for next := range toQuery {
				response, err := s.query(next.Connector(), store)
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

func (s *storer) query(pConn peer.Connector, store *Store) (*api.StoreResponse, error) {
	storeClient, err := s.storerCreator.Create(pConn)
	if err != nil {
		return nil, err
	}
	ctx, cancel, err := client.NewSignedTimeoutContext(s.signer, store.Request,
		store.Params.Timeout)
	if err != nil {
		return nil, err
	}
	retryStoreClient := client.NewRetryStorer(storeClient, storerStoreRetryTimeout)
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
	next := store.Result.Unqueried[0]
	store.Result.Unqueried = store.Result.Unqueried[1:]
	return next
}

func sendNextToQuery(toQuery chan peer.Peer, store *Store) bool {
	next := getNextToQuery(store)
	if next != nil {
		toQuery <- next
	}
	return store.Finished()
}

func processAnyReponse(pr *peerResponse, store *Store) (bool, bool) {
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
		pr.peer.Recorder().Record(peer.Response, peer.Error)
		errored = true
	} else {
		store.wrapLock(func() {
			store.Result.Responded = append(store.Result.Responded, pr.peer)
		})
		pr.peer.Recorder().Record(peer.Response, peer.Success)
	}
	return errored, store.Finished()
}
