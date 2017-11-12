package store

import (
	"bytes"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
)

const storerStoreRetryTimeout = 25 * time.Millisecond

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

func (s *storer) Store(store *Store, seeds []peer.Peer) error {
	if err := s.searcher.Search(store.Search, seeds); err != nil {
		store.Result = NewFatalResult(err)
		return err
	}
	store.Result = NewInitialResult(store.Search.Result)

	// queue of peers to send Store requests to
	storePeers := make(chan peer.Peer, store.Params.NReplicas)

	// load queue with initial peers to send requests to
	initialStorePeers := store.Result.Unqueried[:store.Params.NReplicas]
	store.Result.Unqueried = store.Result.Unqueried[store.Params.NReplicas:]

	for i := 0; i < len(initialStorePeers); i++ {
		storePeers <- initialStorePeers[i]
	}
	finished := make(chan struct{})

	for c := uint(0); c < store.Params.Concurrency; c++ {
		go func() {
			for next := range storePeers {
				ok := s.storePeer(store, next)
				if store.Finished() {
					store.wrapLock(func() { maybeClose(finished) })
					break
				} else if !ok && store.moreUnqueried() {
					// add another peer to queue to take spot of one that just failed
					store.wrapLock(func() {
						storePeers <- store.Result.Unqueried[0]
						store.Result.Unqueried = store.Result.Unqueried[1:]
					})
				}
			}
		}()
	}

	select {
	case <-finished:
		close(storePeers)
	}
	return store.Result.FatalErr
}

func maybeClose(finished chan struct{}) {
	select {
	case <- finished:
	default:
		close(finished)
	}
}

func (s *storer) storePeer(store *Store, next peer.Peer) bool {

	// do the query
	if _, err := s.query(next.Connector(), store); err != nil {
		// if we had an issue querying, skip to next peer
		store.wrapLock(func() {
			store.Result.Errors = append(store.Result.Errors, err)
			next.Recorder().Record(peer.Response, peer.Error)
		})
		return false
	}
	store.wrapLock(func() {
		store.Result.Responded = append(store.Result.Responded, next)
		next.Recorder().Record(peer.Response, peer.Success)
	})
	return true
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
