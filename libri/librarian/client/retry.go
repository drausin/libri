package client

import (
	"github.com/drausin/libri/libri/librarian/api"
	"time"
	"google.golang.org/grpc"
	cbackoff "github.com/cenkalti/backoff"
	"golang.org/x/net/context"
)

const (
	defaultExpBackoffInitialInterval     = 10 * time.Millisecond
	defaultExpBackoffRandomizationFactor = 0.25
	defaultExpBackoffMultiplier          = 1.414
	defaultExpBackoffMaxInterval         = 250 * time.Millisecond
)

type retryGetter struct {
	cb      api.ClientBalancer
	timeout time.Duration
}

func NewRetryGetter(cb api.ClientBalancer, timeout time.Duration) api.Getter {
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
	cb      api.ClientBalancer
	timeout time.Duration
}

func NewRetryPutter(cb api.ClientBalancer, timeout time.Duration) api.Putter {
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
		if err != nil {
			_, err = lc.Ping(ctx, &api.PingRequest{})
		}
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
