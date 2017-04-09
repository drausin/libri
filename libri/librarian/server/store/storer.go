package store

import (
	"bytes"
	"sync"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
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
	querier client.StoreQuerier
}

// NewStorer creates a new Storer instance with given Searcher and StoreQuerier instances.
func NewStorer(signer client.Signer, searcher search.Searcher, q client.StoreQuerier) Storer {
	return &storer{
		signer:   signer,
		searcher: searcher,
		querier:  q,
	}
}

// NewDefaultStorer creates a new Storer with default Searcher and StoreQuerier instances.
func NewDefaultStorer(peerID ecid.ID) Storer {
	signer := client.NewSigner(peerID.Key())
	return NewStorer(
		signer,
		search.NewDefaultSearcher(signer),
		client.NewStoreQuerier(),
	)
}

func (s *storer) Store(store *Store, seeds []peer.Peer) error {
	if err := s.searcher.Search(store.Search, seeds); err != nil {
		store.Result.FatalErr = err
		return store.Result.FatalErr
	}
	store.Result = NewInitialResult(store.Search.Result)

	var wg sync.WaitGroup
	for c := uint(0); c < store.Params.Concurrency; c++ {
		wg.Add(1)
		go s.storeWork(store, &wg)
	}
	wg.Wait()

	return store.Result.FatalErr
}

func (s *storer) storeWork(store *Store, wg *sync.WaitGroup) {
	defer wg.Done()
	// work is finished when either the store is finished or we have no more unqueried peers
	// (but the final, remaining queried peers may not have responded yet)
	for !store.Finished() && store.safeMoreUnqueried() {

		// get next peer to query
		store.mu.Lock()
		if !store.moreUnqueried() {
			// also check for empty unqueried peers here in case anything has changed
			// since loop condition
			store.mu.Unlock()
			break
		}
		next := store.Result.Unqueried[0]
		store.Result.Unqueried = store.Result.Unqueried[1:]
		store.mu.Unlock()
		if _, err := next.Connector().Connect(); err != nil {
			// if we have issues connecting, skip to next peer
			continue
		}

		// do the query
		_, err := s.query(next.Connector(), store)
		if err != nil {
			// if we had an issue querying, skip to next peer
			store.wrapLock(func() {
				store.Result.NErrors++
				next.Recorder().Record(peer.Response, peer.Error)
			})
			continue
		}
		store.wrapLock(func() { next.Recorder().Record(peer.Response, peer.Success) })

		// add to slice of responded peers
		store.wrapLock(func() {
			store.Result.Responded = append(store.Result.Responded, next)
		})
	}
}

func (s *storer) query(pConn peer.Connector, store *Store) (*api.StoreResponse, error) {
	ctx, cancel, err := client.NewSignedTimeoutContext(s.signer, store.Request,
		store.Params.Timeout)
	defer cancel()
	if err != nil {
		return nil, err
	}

	rp, err := s.querier.Query(ctx, pConn, store.Request)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, store.Request.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}

	return rp, nil
}
