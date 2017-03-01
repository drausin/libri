package client

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// IntroduceQuerier handles Introduce queries to a peer.
type IntroduceQuerier interface {
	// Query uses a peer connection to make an Introduce query and returns its response.
	Query(ctx context.Context, pConn peer.Connector, rq *api.IntroduceRequest,
		opts ...grpc.CallOption) (*api.IntroduceResponse, error)
}

type introQuerier struct{}

// NewIntroduceQuerier creates a new IntroduceQuerier.
func NewIntroduceQuerier() IntroduceQuerier {
	return &introQuerier{}
}

func (q *introQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.IntroduceRequest,
	opts ...grpc.CallOption) (*api.IntroduceResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Introduce(ctx, rq, opts...)
}

// FindQuerier handles Find queries to a peer.
type FindQuerier interface {
	// Query uses a peer connection to query for a particular key with an api.FindRequest and
	// returns its response.
	Query(ctx context.Context, pConn peer.Connector, rq *api.FindRequest,
		opts ...grpc.CallOption) (*api.FindResponse, error)
}

type findQuerier struct{}

// NewFindQuerier creates a new FindQuerier instance for FindPeers queries.
func NewFindQuerier() FindQuerier {
	return &findQuerier{}
}

func (q *findQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Find(ctx, rq, opts...)
}

// StoreQuerier handle Store queries to a peer
type StoreQuerier interface {
	// Query uses a peer connection to make a store request.
	Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
		opts ...grpc.CallOption) (*api.StoreResponse, error)
}

type storeQuerier struct{}

// NewStoreQuerier creates a new Querier instance for Store queries.
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
