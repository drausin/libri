package client

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"math/rand"
	"github.com/pkg/errors"
	"time"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestRetryGetter_Get_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	doc, _ := api.NewTestDocument(rng)

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.GetterBalancer{

		// case 0
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{responseValue: doc}, // first call succeeds
			},
		},

		// case 1
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{err: errors.New("some Get error")}, // first call fails
				&fixedGetter{responseValue: doc},                // second call succeeds
			},
		},

		// case 2
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{err: errors.New("some Get error")}, // first call fails
				&fixedGetter{err: errors.New("some Get error")}, // second call fails
				&fixedGetter{responseValue: doc},                // third call succeeds
			},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryGetter(c, timeout)
		rp, err := rg.Get(nil, nil) // since .Get() is mocked, inputs don't matter
		assert.Nil(t, err, info)
		assert.Equal(t, doc, rp.Value, info)
	}
}

func TestRetryGetter_Get_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	doc, _ := api.NewTestDocument(rng)

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.GetterBalancer{
		// case 0
		&fixedGetterBalancer{
			err: errors.New("some Next error"),
		},

		// case 1
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{err: errors.New("some Get error")},
			},
		},

		// case 2
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{err: errors.New("some Get error")},
				&fixedGetter{err: errors.New("some Get error")},
			},
		},

		// case 3
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{err: errors.New("some Get error")},
				&fixedGetter{err: errors.New("some Get error")},
				&fixedGetter{err: errors.New("some Get error")},
			},
		},

		// case 4
		&fixedGetterBalancer{
			clients: []api.Getter{
				&fixedGetter{
					sleep: 200 * time.Millisecond,  // will trigger timeout before next Get
					err: errors.New("some Get error"),
				},
				&fixedGetter{
					responseValue: doc,
				},
			},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryGetter(c, timeout)
		rp, err := rg.Get(nil, nil) // since .Get() is mocked, inputs don't matter
		assert.NotNil(t, err, info)
		assert.Nil(t, rp, info)
	}
}

func TestRetryPutter_Put_ok(t *testing.T) {
	timeout := 100 * time.Millisecond
	response := &api.PutResponse{NReplicas: 3}

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.PutterBalancer{

		// case 0
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{response: response}, // first call succeeds
			},
		},

		// case 1
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{err: errors.New("some Put error")}, // first call fails
				&fixedPutter{response: response},                // second call succeeds
			},
		},

		// case 2
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{err: errors.New("some Put error")}, // first call fails
				&fixedPutter{err: errors.New("some Put error")}, // second call fails
				&fixedPutter{response: response},                // third call succeeds
			},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryPutter(c, timeout)
		rp, err := rg.Put(nil, nil) // since .Put() is mocked, inputs don't matter
		assert.Nil(t, err, info)
		assert.Equal(t, response, rp, info)
	}
}

func TestRetryPutter_Put_err(t *testing.T) {
	timeout := 100 * time.Millisecond
	response := &api.PutResponse{NReplicas: 3}

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.PutterBalancer{
		// case 0
		&fixedPutterBalancer{
			err: errors.New("some Next error"),
		},

		// case 1
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{err: errors.New("some Put error")},
			},
		},

		// case 2
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{err: errors.New("some Put error")},
				&fixedPutter{err: errors.New("some Put error")},
			},
		},

		// case 3
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{err: errors.New("some Put error")},
				&fixedPutter{err: errors.New("some Put error")},
				&fixedPutter{err: errors.New("some Put error")},
			},
		},

		// case 4
		&fixedPutterBalancer{
			clients: []api.Putter{
				&fixedPutter{
					sleep: 200 * time.Millisecond,  // will trigger timeout before next Put
					err: errors.New("some Put error"),
				},
				&fixedPutter{
					response: response,
				},
			},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryPutter(c, timeout)
		rp, err := rg.Put(nil, nil) // since .Put() is mocked, inputs don't matter
		assert.NotNil(t, err, info)
		assert.Nil(t, rp, info)
	}
}

type fixedGetterBalancer struct {
	clients []api.Getter
	err     error
}

func (f *fixedGetterBalancer) Next() (api.Getter, error) {
	if len(f.clients) == 0 {
		return nil, errors.New("out of Getters")
	}
	next := f.clients[0]
	f.clients = f.clients[1:]
	return next, f.err
}

type fixedPutterBalancer struct {
	clients []api.Putter
	err    error
}

func (f *fixedPutterBalancer) Next() (api.Putter, error) {
	if len(f.clients) == 0 {
		return nil, errors.New("out of Putters")
	}
	next := f.clients[0]
	f.clients = f.clients[1:]
	return next, f.err
}

type fixedGetter struct {
	responseValue *api.Document
	sleep         time.Duration
	err           error
}

func (f *fixedGetter) Get(ctx context.Context, in *api.GetRequest, opts ...grpc.CallOption) (
	*api.GetResponse, error) {
	time.Sleep(f.sleep)
	return &api.GetResponse{Value: f.responseValue}, f.err
}

type fixedPutter struct {
	response *api.PutResponse
	sleep         time.Duration
	err     error
}

func (f *fixedPutter) Put(
	ctx context.Context, in *api.PutRequest, opts ...grpc.CallOption,
) (*api.PutResponse, error) {
	time.Sleep(f.sleep)
	return f.response, f.err
}
