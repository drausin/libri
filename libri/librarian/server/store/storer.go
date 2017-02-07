package store

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Storer interface {
	Store(store *Store, seeds []peer.Peer) error
}

type storer struct {
	// searcher is used for the first search half of the store operation
	searcher search.Searcher

	// issues store queries to the peers
	q StoreQuerier
}

func NewStorer(searcher search.Searcher, q StoreQuerier) Storer {
	return &storer{
		searcher: searcher,
		q:        q,
	}
}

func NewDefaultStorer() Storer {
	return NewStorer(
		search.NewDefaultSearcher(),
		NewStoreQuerier(),
	)
}

func (s *storer) Store(store *Store, seeds []peer.Peer) error {
	if err := s.searcher.Search(store.Search, seeds); err != nil {
		store.FatalErr = err
		return store.FatalErr
	}
	store.Result = NewInitialResult(store.Search.Result)

	var wg sync.WaitGroup
	for c := uint(0); c < store.Params.Concurrency; c++ {
		wg.Add(1)
		go s.storeWork(store, &wg)
	}
	wg.Wait()

	return store.FatalErr
}

func (s *storer) storeWork(store *Store, wg *sync.WaitGroup) {
	defer wg.Done()
	for !store.Finished() {

		// get next peer to query
		store.mu.Lock()
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
			store.mu.Lock()
			store.NErrors++
			store.mu.Unlock()
			next.Responses().Error()
			continue
		}
		next.Responses().Success()

		// add to slice of responded peers
		store.mu.Lock()
		store.Result.Responded = append(store.Result.Responded, next)
		store.mu.Unlock()
	}
}

func (s *storer) query(pConn peer.Connector, store *Store) (*api.StoreResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), store.Params.Timeout)
	defer cancel()

	rp, err := s.q.Query(ctx, pConn, store.Request)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.RequestId, store.Request.RequestId) {
		return nil, fmt.Errorf("unexpected response request ID received: %v, "+
			"expected %v", rp.RequestId, store.Request.RequestId)
	}

	return rp, nil
}

// StoreQuerier handle Store queries to a peer
type StoreQuerier interface {
	// Query uses a peer connection to make a store request.
	Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
		opts ...grpc.CallOption) (*api.StoreResponse, error)
}

type storeQuerier struct{}

// NewQuerier creates a new Querier instance for Store queries.
func NewStoreQuerier() StoreQuerier {
	return &storeQuerier{}
}

func (q *storeQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Store(ctx, rq, opts...)
}
