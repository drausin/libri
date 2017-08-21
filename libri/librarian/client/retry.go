package client

import (
	"time"

	cbackoff "github.com/cenkalti/backoff"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	defaultExpBackoffInitialInterval     = 10 * time.Millisecond
	defaultExpBackoffRandomizationFactor = 0.25
	defaultExpBackoffMultiplier          = 1.414
	defaultExpBackoffMaxInterval         = 250 * time.Millisecond
)

var (
	// ErrGetMissingValue denotes when a Get request expectes to receive a value response but
	// doesn't and instead receives a ClosestPeers response.
	ErrGetMissingValue = errors.New("Get response expected to have value but it is missing")
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

	backoff := NewExpBackoff(r.timeout)
	if err := cbackoff.Retry(operation, backoff); err != nil {
		return nil, err
	}
	return rp, nil
}

type retryVerifier struct {
	inner   api.Verifier
	timeout time.Duration
}

// NewRetryVerifier creates a new api.Finder with exponential backoff retries.
func NewRetryVerifier(inner api.Verifier, timeout time.Duration) api.Verifier {
	return &retryVerifier{
		inner:   inner,
		timeout: timeout,
	}
}

func (r *retryVerifier) Verify(
	ctx context.Context, rq *api.VerifyRequest, opts ...grpc.CallOption,
) (*api.VerifyResponse, error) {

	var rp *api.VerifyResponse
	operation := func() error {
		var err error
		rp, err = r.inner.Verify(ctx, rq, opts...)
		return err
	}

	backoff := NewExpBackoff(r.timeout)
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

	backoff := NewExpBackoff(r.timeout)
	if err := cbackoff.Retry(operation, backoff); err != nil {
		return nil, err
	}
	return rp, nil
}

type retryGetter struct {
	cb        GetterBalancer
	valueOnly bool
	timeout   time.Duration
}

// NewRetryGetter wraps a client balancer with an exponential backoff, returning an api.Getter. Each
// backoff attempt samples a (possibly) different api.Getter to use for the query.
func NewRetryGetter(cb GetterBalancer, valueOnly bool, timeout time.Duration) api.Getter {
	return &retryGetter{
		cb:        cb,
		valueOnly: valueOnly,
		timeout:   timeout,
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
		if err != nil {
			return err
		}
		if rp.Value == nil && r.valueOnly {
			return ErrGetMissingValue
		}
		return nil
	}
	if err := cbackoff.Retry(operation, NewExpBackoff(r.timeout)); err != nil {
		return nil, err
	}
	return rp, nil
}

type retryPutter struct {
	cb      PutterBalancer
	timeout time.Duration
}

// NewRetryPutter wraps a client balancer with an exponential backoff, returning an api.Putter.
func NewRetryPutter(cb PutterBalancer, timeout time.Duration) api.Putter {
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
	if err := cbackoff.Retry(operation, NewExpBackoff(r.timeout)); err != nil {
		return nil, err
	}
	return rp, nil
}

func NewExpBackoff(timeout time.Duration) *cbackoff.ExponentialBackOff {
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
