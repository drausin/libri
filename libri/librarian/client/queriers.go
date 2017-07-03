package client

import (
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
	cbackoff "github.com/cenkalti/backoff"
	"google.golang.org/grpc"
	"time"
)

// TODO (drausin) move all queriers to api.(Finder|Storer|Getter|Putter)...?

// IntroduceQuerier issues Introduce queries to a peer.
type IntroduceQuerier interface {
	// Query uses a peer connection to make an Introduce query and returns its response.
	Query(ctx context.Context, pConn api.Connector, rq *api.IntroduceRequest,
		opts ...grpc.CallOption) (*api.IntroduceResponse, error)
}

type introQuerier struct{}

// NewIntroduceQuerier creates a new IntroduceQuerier.
func NewIntroduceQuerier() IntroduceQuerier {
	return &introQuerier{}
}

func (q *introQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.IntroduceRequest,
	opts ...grpc.CallOption) (*api.IntroduceResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Introduce(ctx, rq, opts...)
}

// FindQuerier issues Find queries to a peer.
type FindQuerier interface {
	// Query uses a peer connection to query for a particular key with an api.FindRequest and
	// returns its response.
	Query(ctx context.Context, pConn api.Connector, rq *api.FindRequest,
		opts ...grpc.CallOption) (*api.FindResponse, error)
}

type findQuerier struct{}

// NewFindQuerier creates a new FindQuerier instance for FindPeers queries.
func NewFindQuerier() FindQuerier {
	return &findQuerier{}
}

func (q *findQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Find(ctx, rq, opts...)
}

type retryFindQuerier struct {
	timeout time.Duration
	inner FindQuerier
}

// NewRetryFindQuerier creates a new FindQuerier instance that retries a query with exponential
// backoff until a given timeout.
func NewRetryFindQuerier(inner FindQuerier, timeout time.Duration) FindQuerier {
	return &retryFindQuerier{
		inner: inner,
		timeout: timeout,
	}
}

func (r * retryFindQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	var rp *api.FindResponse
	operation := func() error {
		var err error
		rp, err = r.inner.Query(ctx, pConn, rq)
		return err
	}

	backoff := newExpBackoff(r.timeout)
	if err := cbackoff.Retry(operation, backoff); err != nil {
		return nil, err
	}
	return rp, nil
}

// StoreQuerier issues Store queries to a peer.
type StoreQuerier interface {
	// Query uses a peer connection to make a Store request.
	Query(ctx context.Context, pConn api.Connector, rq *api.StoreRequest,
		opts ...grpc.CallOption) (*api.StoreResponse, error)
}

type storeQuerier struct{}

// NewStoreQuerier creates a new StoreQuerier instance.
func NewStoreQuerier() StoreQuerier {
	return &storeQuerier{}
}

func (q *storeQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Store(ctx, rq, opts...)
}

// GetQuerier issues Get queries to a peer.
type GetQuerier interface {
	// Query uses a peer connection to make a Get request.
	Query(ctx context.Context, pConn api.Connector, rq *api.GetRequest,
		opts ...grpc.CallOption) (*api.GetResponse, error)
}

type getQuerier struct{}

// NewGetQuerier returns a new GetQuerier instance.
func NewGetQuerier() GetQuerier {
	return &getQuerier{}
}

func (q *getQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.GetRequest,
	opts ...grpc.CallOption) (*api.GetResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Get(ctx, rq, opts...)
}

// PutQuerier issues Get queries to a peer.
type PutQuerier interface {
	// Query uses a peer connection to make a Put request.
	Query(ctx context.Context, pConn api.Connector, rq *api.PutRequest,
		opts ...grpc.CallOption) (*api.PutResponse, error)
}

type putQuerier struct{}

// NewPutQuerier returns a new PutQuerier instance.
func NewPutQuerier() PutQuerier {
	return &putQuerier{}
}

func (q *putQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.PutRequest,
	opts ...grpc.CallOption) (*api.PutResponse, error) {
	client, err := pConn.Connect() // *should* be already connected, but do here just in case
	if err != nil {
		return nil, err
	}
	return client.Put(ctx, rq, opts...)
}
