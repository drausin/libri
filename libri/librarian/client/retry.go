package client

import (
	"time"

	cbackoff "github.com/cenkalti/backoff"
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	defaultExpBackoffInitialInterval     = 10 * time.Millisecond
	defaultExpBackoffRandomizationFactor = 0.25
	defaultExpBackoffMultiplier          = 1.414
	defaultExpBackoffMaxInterval         = 250 * time.Millisecond
)

type retryFinder struct {
	inner   api.Finder
	timeout time.Duration
}

// NewRetryFinder creates a new api.Finder with exponential backoff retries.
func NewRetryFinder(inner api.Finder, timeout time.Duration) api.Finder {
	return &retryFinder{
		inner:   inner,
		timeout: timeout,
	}
}

func (r *retryFinder) Find(ctx context.Context, rq *api.FindRequest, opts ...grpc.CallOption) (
	*api.FindResponse, error) {
	var rp *api.FindResponse
	operation := func() error {
		var err error
		rp, err = r.inner.Find(ctx, rq, opts...)
		return err
	}

	backoff := newExpBackoff(r.timeout)
	if err := cbackoff.Retry(operation, backoff); err != nil {
		return nil, err
	}
	return rp, nil
}

type retryStorer struct {
	inner   api.Storer
	timeout time.Duration
}

// NewRetryStorer creates a new api.Storer with exponential backoff retries.
func NewRetryStorer(inner api.Storer, timeout time.Duration) api.Storer {
	return &retryStorer{
		inner:   inner,
		timeout: timeout,
	}
}

func (r *retryStorer) Store(ctx context.Context, rq *api.StoreRequest, opts ...grpc.CallOption) (
	*api.StoreResponse, error) {

	var rp *api.StoreResponse
	operation := func() error {
		var err error
		rp, err = r.inner.Store(ctx, rq, opts...)
		return err
	}

	backoff := newExpBackoff(r.timeout)
	if err := cbackoff.Retry(operation, backoff); err != nil {
		return nil, err
	}
	return rp, nil
}

type retryGetter struct {
	cb      api.GetterBalancer
	timeout time.Duration
}

// NewRetryGetter wraps a client balancer with an exponential backoff, returning an api.Getter. Each
// backoff attempt samples a (possibly) different api.Getter to use for the query.
func NewRetryGetter(cb api.GetterBalancer, timeout time.Duration) api.Getter {
	return &retryGetter{
		cb:      cb,
		timeout: timeout,
	}
}

func (r *retryGetter) Get(ctx context.Context, in *api.GetRequest, opts ...grpc.CallOption) (
	*api.GetResponse, error) {

	var rp *api.GetResponse
	operation := func() error {
		var err error
		lc, err := r.cb.Next()
		if err != nil {
			return err
		}
		rp, err = lc.Get(ctx, in, opts...)
		return err
	}
	if err := cbackoff.Retry(operation, newExpBackoff(r.timeout)); err != nil {
		return nil, err
	}
	return rp, nil
}

type retryPutter struct {
	cb      api.PutterBalancer
	timeout time.Duration
}

// NewRetryPutter wraps a client balancer with an exponential backoff, returning an api.Putter.
func NewRetryPutter(cb api.PutterBalancer, timeout time.Duration) api.Putter {
	return &retryPutter{
		cb:      cb,
		timeout: timeout,
	}
}

func (r *retryPutter) Put(ctx context.Context, in *api.PutRequest, opts ...grpc.CallOption) (
	*api.PutResponse, error) {

	var rp *api.PutResponse
	operation := func() error {
		var err error
		lc, err := r.cb.Next()
		if err != nil {
			return err
		}
		rp, err = lc.Put(ctx, in, opts...)
		return err
	}
	if err := cbackoff.Retry(operation, newExpBackoff(r.timeout)); err != nil {
		return nil, err
	}
	return rp, nil
}

func newExpBackoff(timeout time.Duration) *cbackoff.ExponentialBackOff {
	b := &cbackoff.ExponentialBackOff{
		InitialInterval:     defaultExpBackoffInitialInterval,
		RandomizationFactor: defaultExpBackoffRandomizationFactor,
		Multiplier:          defaultExpBackoffMultiplier,
		MaxInterval:         defaultExpBackoffMaxInterval,
		MaxElapsedTime:      timeout,
		Clock:               cbackoff.SystemClock,
	}
	b.Reset()
	return b
}
