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
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/drausin/libri/libri/librarian/server/ecid"
)

// Storer executes store operations.
type Storer interface {
	// Store executes a store operation, starting with a given set of seed peers.
	Store(store *Store, seeds []peer.Peer) error
}

type storer struct {
	// signs queries
	signer   signature.Signer

	// searcher is used for the first search half of the store operation
	searcher search.Searcher

	// issues store queries to the peers
	q        Querier
}

// NewStorer creates a new Storer instance with given Searcher and StoreQuerier instances.
func NewStorer(signer signature.Signer, searcher search.Searcher, q Querier) Storer {
	return &storer{
		signer: signer,
		searcher: searcher,
		q:        q,
	}
}

// NewDefaultStorer creates a new Storer with default Searcher and StoreQuerier instances.
func NewDefaultStorer(peerID ecid.ID) Storer {
	return NewStorer(
		signature.NewSigner(peerID.Key()),
		search.NewDefaultSearcher(peerID),
		NewQuerier(),
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
	for !store.Finished() && len(store.Result.Unqueried) > 0 {

		// get next peer to query
		store.mu.Lock()
		if len(store.Result.Unqueried) == 0 {
			// also check for empty unqueried peers here in case anythin has changed
			// since loop condition
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
			store.mu.Lock()
			store.Result.NErrors++
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
	if !bytes.Equal(rp.Metadata.RequestId, store.Request.Metadata.RequestId) {
		return nil, fmt.Errorf("unexpected response request ID received: %v, "+
			"expected %v", rp.Metadata.RequestId, store.Request.Metadata.RequestId)
	}

	return rp, nil
}

func (s *storer) context(store *Store) (context.Context,  context.CancelFunc, error) {
	ctx := context.Background()

	// sign the message
	signedJWT, err := s.signer.Sign(store.Request)
	if err != nil {
		return nil, nil, err
	}
	ctx = context.WithValue(ctx, signature.ContextKey, signedJWT)

	// add timeout
	ctx, cancel := context.WithTimeout(ctx, store.Params.Timeout)

	return ctx, cancel, nil
}

// Querier handle Store queries to a peer
type Querier interface {
	// Query uses a peer connection to make a store request.
	Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
		opts ...grpc.CallOption) (*api.StoreResponse, error)
}

type querier struct{}

// NewQuerier creates a new Querier instance for Store queries.
func NewQuerier() Querier {
	return &querier{}
}

func (q *querier) Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Store(ctx, rq, opts...)
}
