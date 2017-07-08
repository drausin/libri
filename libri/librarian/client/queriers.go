package client

import (
	"time"

	cbackoff "github.com/cenkalti/backoff"
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// TODO (drausin) move all queriers to api.(Finder|Storer|Getter|Putter)...?

// IntroducerCreator creates api.Introducers.
type IntroducerCreator interface {
	// Create creates an api.Introducer from the api.Connector.
	Create(conn api.Connector) (api.Introducer, error)
}

type introducerCreator struct {}

// NewIntroducerCreator creates a new IntroducerCreator.
func NewIntroducerCreator() IntroducerCreator {
	return &introducerCreator{}
}

func (*introducerCreator) Create(c api.Connector) (api.Introducer, error) {
	lc, err := c.Connect()
	if err != nil {
		return nil, err
	}
	return lc.(api.Introducer), nil
}

// FinderCreator creates api.Finders.
type FinderCreator interface {
	// Create creates an api.Finder from the api.Connector.
	Create(conn api.Connector) (api.Finder, error)
}

type finderCreator struct {}

// NewFinderCreator creates a new FinderCreator.
func NewFinderCreator() IntroducerCreator {
	return &introducerCreator{}
}

func (*finderCreator) Create(c api.Connector) (api.Finder, error) {
	lc, err := c.Connect()
	if err != nil {
		return nil, err
	}
	return lc.(api.Finder), nil
}

type retryFindQuerier struct {
	timeout time.Duration
	inner   FindQuerier
}

// NewRetryFindQuerier creates a new FindQuerier instance that retries a query with exponential
// backoff until a given timeout.
func NewRetryFindQuerier(inner FindQuerier, timeout time.Duration) FindQuerier {
	return &retryFindQuerier{
		inner:   inner,
		timeout: timeout,
	}
}

func (r *retryFindQuerier) Query(ctx context.Context, pConn api.Connector, rq *api.FindRequest,
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
