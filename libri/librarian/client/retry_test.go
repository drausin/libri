package client

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestRetryFinder_Find_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	doc, _ := api.NewTestDocument(rng)
	err := errors.New("some Find error")

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.Finder{

		// case 0
		&fixedFinder{ // first call succeeds
			responses: []*api.FindResponse{{Value: doc}},
			errs:      []error{nil},
		},

		// case 1
		&fixedFinder{ // first call fails; second call succeeds
			responses: []*api.FindResponse{nil, {Value: doc}},
			errs:      []error{err, nil},
		},

		// case 2
		&fixedFinder{ // first & second calls fail; third call succeeds
			responses: []*api.FindResponse{nil, nil, {Value: doc}},
			errs:      []error{err, err, nil},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryFinder(c, timeout)
		rp, err := rg.Find(nil, nil) // since .Find() is mocked, inputs don't matter
		assert.Nil(t, err, info)
		assert.Equal(t, doc, rp.Value, info)
	}
}

func TestRetryFinder_Find_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	doc, _ := api.NewTestDocument(rng)
	err := errors.New("some Find error")

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.Finder{

		// case 0
		&fixedFinder{
			responses: []*api.FindResponse{nil},
			errs:      []error{err},
		},

		// case 1
		&fixedFinder{
			responses: []*api.FindResponse{nil, nil},
			errs:      []error{err, err},
		},

		// case 2
		&fixedFinder{
			responses: []*api.FindResponse{nil, nil, nil},
			errs:      []error{err, err, err},
		},

		// case 3
		&fixedFinder{
			responses: []*api.FindResponse{nil, {Value: doc}},
			errs:      []error{err, nil},
			sleep:     200 * time.Millisecond, // will trigger timeout before next Find
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryFinder(c, timeout)
		rp, err := rg.Find(nil, nil) // since .Find() is mocked, inputs don't matter
		assert.NotNil(t, err, info)
		assert.Nil(t, rp, info)
	}
}

func TestRetryVerifier_Verify_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	mac := api.RandBytes(rng, 32)
	err := errors.New("some Verify error")

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.Verifier{

		// case 0
		&fixedVerifier{ // first call succeeds
			responses: []*api.VerifyResponse{{Mac: mac}},
			errs:      []error{nil},
		},

		// case 1
		&fixedVerifier{ // first call fails; second call succeeds
			responses: []*api.VerifyResponse{nil, {Mac: mac}},
			errs:      []error{err, nil},
		},

		// case 2
		&fixedVerifier{ // first & second calls fail; third call succeeds
			responses: []*api.VerifyResponse{nil, nil, {Mac: mac}},
			errs:      []error{err, err, nil},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryVerifier(c, timeout)
		rp, err := rg.Verify(nil, nil) // since Verify() is mocked, inputs don't matter
		assert.Nil(t, err, info)
		assert.Equal(t, mac, rp.Mac, info)
	}
}

func TestRetryVerifier_Verify_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	mac := api.RandBytes(rng, 32)
	err := errors.New("some Find error")

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.Verifier{

		// case 0
		&fixedVerifier{
			responses: []*api.VerifyResponse{nil},
			errs:      []error{err},
		},

		// case 1
		&fixedVerifier{
			responses: []*api.VerifyResponse{nil, nil},
			errs:      []error{err, err},
		},

		// case 2
		&fixedVerifier{
			responses: []*api.VerifyResponse{nil, nil, nil},
			errs:      []error{err, err, err},
		},

		// case 3
		&fixedVerifier{
			responses: []*api.VerifyResponse{nil, {Mac: mac}},
			errs:      []error{err, nil},
			sleep:     200 * time.Millisecond, // will trigger timeout before next Find
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryVerifier(c, timeout)
		rp, err := rg.Verify(nil, nil) // since Verify() is mocked, inputs don't matter
		assert.NotNil(t, err, info)
		assert.Nil(t, rp, info)
	}
}

func TestRetryStorer_Store_ok(t *testing.T) {
	timeout := 100 * time.Millisecond
	err := errors.New("some Store error")

	// check each case ultimately succeeds despite possible initial failures
	cases := []api.Storer{

		// case 0
		&fixedStorer{ // first call succeeds
			responses: []*api.StoreResponse{{}},
			errs:      []error{nil},
		},

		// case 1
		&fixedStorer{ // first call fails; second call succeeds
			responses: []*api.StoreResponse{nil, {}},
			errs:      []error{err, nil},
		},

		// case 2
		&fixedStorer{ // first & second calls fail; third call succeeds
			responses: []*api.StoreResponse{nil, nil, {}},
			errs:      []error{err, err, nil},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryStorer(c, timeout)
		rp, err := rg.Store(nil, nil) // since .Store() is mocked, inputs don't matter
		assert.Nil(t, err, info)
		assert.Equal(t, &api.StoreResponse{}, rp, info)
	}
}

func TestRetryStorer_Store_err(t *testing.T) {
	timeout := 100 * time.Millisecond
	err := errors.New("some Stor error")

	// check each case ultimately fails
	cases := []api.Storer{

		// case 0
		&fixedStorer{
			responses: []*api.StoreResponse{{}},
			errs:      []error{err},
		},

		// case 1
		&fixedStorer{
			responses: []*api.StoreResponse{{}, {}},
			errs:      []error{err, err},
		},

		// case 2
		&fixedStorer{
			responses: []*api.StoreResponse{{}, {}, {}},
			errs:      []error{err, err, err},
		},

		// case 3
		&fixedStorer{
			responses: []*api.StoreResponse{{}, {}},
			errs:      []error{err, nil},
			sleep:     200 * time.Millisecond, // will trigger timeout before next Stor
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rg := NewRetryStorer(c, timeout)
		rp, err := rg.Store(nil, nil) // since .Store() is mocked, inputs don't matter
		assert.NotNil(t, err, info)
		assert.Nil(t, rp, info)
	}
}

func TestRetryGetter_Get_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	timeout := 100 * time.Millisecond
	doc, _ := api.NewTestDocument(rng)

	// check each case ultimately succeeds despite possible initial failures
	cases := []GetterBalancer{

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

	// check each case ultimately fails
	cases := []GetterBalancer{
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
					sleep: 200 * time.Millisecond, // will trigger timeout before next Get
					err:   errors.New("some Get error"),
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
	cases := []PutterBalancer{

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

	// check each case ultimately fails
	cases := []PutterBalancer{
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
					sleep: 200 * time.Millisecond, // will trigger timeout before next Put
					err:   errors.New("some Put error"),
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

type fixedFinder struct {
	responses []*api.FindResponse
	sleep     time.Duration
	errs      []error
}

func (f *fixedFinder) Find(ctx context.Context, rq *api.FindRequest, opts ...grpc.CallOption) (
	*api.FindResponse, error) {
	if len(f.responses) == 0 {
		return nil, errors.New("no more responses")
	}
	nextRP := f.responses[0]
	nextErr := f.errs[0]
	f.responses = f.responses[1:]
	f.errs = f.errs[1:]
	time.Sleep(f.sleep)
	return nextRP, nextErr
}

type fixedVerifier struct {
	responses []*api.VerifyResponse
	sleep     time.Duration
	errs      []error
}

func (f *fixedVerifier) Verify(
	ctx context.Context, in *api.VerifyRequest, opts ...grpc.CallOption,
) (*api.VerifyResponse, error) {
	if len(f.responses) == 0 {
		return nil, errors.New("no more responses")
	}
	nextRP := f.responses[0]
	nextErr := f.errs[0]
	f.responses = f.responses[1:]
	f.errs = f.errs[1:]
	time.Sleep(f.sleep)
	return nextRP, nextErr
}

type fixedStorer struct {
	responses []*api.StoreResponse
	sleep     time.Duration
	errs      []error
}

func (f *fixedStorer) Store(ctx context.Context, rq *api.StoreRequest, opts ...grpc.CallOption) (
	*api.StoreResponse, error) {
	if len(f.responses) == 0 {
		return nil, errors.New("no more responses")
	}
	nextRP := f.responses[0]
	nextErr := f.errs[0]
	f.responses = f.responses[1:]
	f.errs = f.errs[1:]
	time.Sleep(f.sleep)
	return nextRP, nextErr
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
	err     error
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
	sleep    time.Duration
	err      error
}

func (f *fixedPutter) Put(
	ctx context.Context, in *api.PutRequest, opts ...grpc.CallOption,
) (*api.PutResponse, error) {
	time.Sleep(f.sleep)
	return f.response, f.err
}
