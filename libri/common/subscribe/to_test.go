package subscribe

import (
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"math/rand"
	"testing"
	"io"
	"github.com/golang/protobuf/proto"
	"sync"
)

func TestTo_BeginEnd(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := NewDefaultParameters()
	params.NSubscriptions = 2
	cb := &fixedClientBalancer{}
	recent, err := NewRecentPublications(2)
	assert.Nil(t, err)
	newPubs := make(chan *KeyedPub, 1)
	end := make(chan struct{})
	toImpl := NewTo(params, nil, cb, nil, recent, newPubs, end).(*to)

	// mock what we actually get from subscriptions
	received := make(chan *pubValueReceipt)
	errs := make(chan error)
	toImpl.sb = &fixedSubscriptionBeginner{
		received: received,
		errs: errs,
	}

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err = toImpl.Begin()
		assert.Nil(t, err)
	}(wg)

	value1 := api.NewTestPublication(rng)
	value2 := api.NewTestPublication(rng)
	value3 := api.NewTestPublication(rng)
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	key2, err := api.GetKey(value2)
	assert.Nil(t, err)
	key3, err := api.GetKey(value3)
	assert.Nil(t, err)
	fromPub1 := api.RandBytes(rng, api.ECPubKeyLength)
	fromPub2 := api.RandBytes(rng, api.ECPubKeyLength)

	// new
	pvr1, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub1)
	assert.Nil(t, err)
	received <- pvr1
	errs <- nil
	newPub, ended := getNewPub(newPubs, end)
	assert.False(t, ended)
	assert.Equal(t, pvr1.pub, newPub)

	// not new
	pvr2, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub2)
	assert.Nil(t, err)
	received <- pvr2
	errs <- nil
	newPub = nil
	select {
	case <- end:
		ended = true
	case newPub = <- newPubs:
	default:
	}
	assert.Nil(t, newPub)
	assert.False(t, ended)

	// new
	pvr3, err := newPublicationValueReceipt(key2.Bytes(), value2, fromPub1)
	assert.Nil(t, err)
	received <- pvr3
	errs <- nil
	newPub, ended = getNewPub(newPubs, end)
	assert.Equal(t, pvr3.pub, newPub)
	assert.False(t, ended)

	// new
	pvr4, err := newPublicationValueReceipt(key3.Bytes(), value3, fromPub2)
	assert.Nil(t, err)
	received <- pvr4
	errs <- nil
	newPub, ended = getNewPub(newPubs, end)
	assert.Equal(t, pvr4.pub, newPub)
	assert.False(t, ended)

	toImpl.End()
	newPub, ended = getNewPub(newPubs, end)
	assert.Nil(t, newPub)
	assert.True(t, ended)

	wg.Wait()
}

func getNewPub(newPubs chan *KeyedPub, end chan struct{}) (newPub *KeyedPub, ended bool) {
	select {
	case <- end:
		ended = true
	case newPub = <- newPubs:
	}
	return newPub, ended
}

func TestTo_Begin_err(t *testing.T) {
	//rng := rand.New(rand.NewSource(0))
	params := NewDefaultParameters()
	params.NSubscriptions = 2
	recent, err := NewRecentPublications(2)
	cb := &fixedClientBalancer{}
	assert.Nil(t, err)
	newPubs := make(chan *KeyedPub, 1)

	// check cb.Next() error bubbles up
	nextErr := errors.New("some Next() error")
	cb1 := &fixedClientBalancer{err: nextErr}
	toImpl1 := NewTo(params, nil, cb1, nil, recent, newPubs, make(chan struct{})).(*to)
	toImpl1.sb = &fixedSubscriptionBeginner{subscribeErr: errors.New("some subscribe error")}
	err = toImpl1.Begin()
	assert.Equal(t, nextErr, err)

	// check NewFPSubscription error bubbles up
	params2 := NewDefaultParameters()
	params2.FPRate = 0.0  // will trigger error
	toImpl2 := NewTo(params2, nil, cb, nil, recent, newPubs, make(chan struct{})).(*to)
	toImpl2.sb = &fixedSubscriptionBeginner{subscribeErr: errors.New("some subscribe error")}
	err = toImpl2.Begin()
	assert.Equal(t, ErrOutOfBoundsFPRate, err)

	// check running error count above threshold triggers error
	received := make(chan *pubValueReceipt)
	errs := make(chan error)
	toImpl3 := NewTo(params, nil, cb, nil, recent, newPubs, make(chan struct{})).(*to)
	toImpl3.sb = &fixedSubscriptionBeginner{
		received: received,
		errs: errs,
		subscribeErr: errors.New("some subscribe error"),
	}
	go func() {
		for c := 0; c < int(params.FPRate * float32(errQueueSize)); c++ {
			received <- nil
			errs <- errors.New("some Recv error")
		}
	}()
	err = toImpl3.Begin()
	assert.Equal(t, ErrTooManySubscriptionErrs, err)
}

func TestSubscriptionBeginnerImpl_Begin_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	fromID := ecid.NewPseudoRandom(rng)
	fromPubKey := ecid.ToPublicKeyBytes(fromID)
	sb := subscriptionBeginnerImpl {
		clientID: clientID,
		signer:   &fixedSigner{signature: "some.signature.jtw"},
		params:   NewDefaultParameters(),
	}
	responses := make(chan *api.SubscribeResponse)
	responseErrs := make(chan error, 1)
	lc := &fixedSubscriber{
		client: &fixedLibrarianSubscribeClient{
			responses: responses,
			err:  responseErrs,
		},
	}
	sub, err := NewFPSubscription(DefaultFPRate, rng)
	assert.Nil(t, err)
	received := make(chan *pubValueReceipt, 1)
	errs := make(chan error)
	end := make(chan struct{})

	go func() {
		err = sb.begin(lc, sub, received, errs, end)
		assert.Nil(t, err)
	}()

	value := api.NewTestPublication(rng)
	key, err := api.GetKey(value)
	assert.Nil(t, err)
	responses <- &api.SubscribeResponse{
		Metadata: &api.ResponseMetadata{
			PubKey: fromPubKey,
		},
		Key:   key.Bytes(),
		Value: value,
	}
	responseErrs <- nil

	receivedPub := <- received
	err = <- errs
	assert.Nil(t, err)
	assert.Equal(t, key, receivedPub.pub.Key)
	assert.Equal(t, value, receivedPub.pub.Value)
	assert.Equal(t, fromPubKey, receivedPub.receipt.fromPub)

	// simulate subscription being close on server side
	responses <- nil
	responseErrs <- io.EOF

	// start again
	go func() {
		err = sb.begin(lc, sub, received, errs, end)
		assert.Nil(t, err)
	}()

	value = api.NewTestPublication(rng)
	key, err = api.GetKey(value)
	assert.Nil(t, err)
	responses <- &api.SubscribeResponse{
		Metadata: &api.ResponseMetadata{
			PubKey: fromPubKey,
		},
		Key:   key.Bytes(),
		Value: value,
	}
	responseErrs <- nil

	// simulate gracefully shutting down subscriber
	close(end)
}

func TestSubscriptionBeginnerImpl_Begin_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	fromID := ecid.NewPseudoRandom(rng)
	fromPubKey := ecid.ToPublicKeyBytes(fromID)
	errs := make(chan error)
	end := make(chan struct{})
	received := make(chan *pubValueReceipt, 1)
	sub, err := NewFPSubscription(DefaultFPRate, rng)
	assert.Nil(t, err)

	// check NewSignedTimeoutContext error bubbles up
	sb1 := subscriptionBeginnerImpl {
		clientID: clientID,
		signer:   &fixedSigner{err: errors.New("some Signer error")},
		params:   NewDefaultParameters(),
	}
	lc1 := &fixedSubscriber{}
	err = sb1.begin(lc1, sub, received, errs, end)
	assert.NotNil(t, err)

	// check Subscribe error bubbles up
	sb2 := subscriptionBeginnerImpl {
		clientID: clientID,
		signer:   &fixedSigner{signature: "some.signature.jtw"},
		params:   NewDefaultParameters(),
	}
	lc2 := &fixedSubscriber{
		client: nil,
		err: errors.New("some Subscribe error"),
	}
	err = sb2.begin(lc2, sub, received, errs, end)
	assert.NotNil(t, err)

	// check Recv error bubbles up
	sb3 := subscriptionBeginnerImpl {
		clientID: clientID,
		signer:   &fixedSigner{signature: "some.signature.jtw"},
		params:   NewDefaultParameters(),
	}
	responses3 := make(chan *api.SubscribeResponse, 1)
	responseErrs3 := make(chan error, 1)
	lc3 := &fixedSubscriber{
		client: &fixedLibrarianSubscribeClient{
			responses: responses3,
			err:  responseErrs3,
		},
	}
	responses3 <- nil
	responseErrs3 <- errors.New("some Recv error")
	err = sb3.begin(lc3, sub, received, errs, end)
	assert.NotNil(t, err)

	// check newPublicationValueReceipt error bubbles up
	sb4 := subscriptionBeginnerImpl {
		clientID: clientID,
		signer:   &fixedSigner{signature: "some.signature.jtw"},
		params:   NewDefaultParameters(),
	}
	responses4 := make(chan *api.SubscribeResponse, 1)
	responseErrs4 := make(chan error, 1)
	lc4 := &fixedSubscriber{
		client: &fixedLibrarianSubscribeClient{
			responses: responses4,
			err:  responseErrs4,
		},
	}
	value := api.NewTestPublication(rng)
	responses4 <- &api.SubscribeResponse{
		Metadata: &api.ResponseMetadata{
			PubKey: fromPubKey,
		},
		Key:   api.RandBytes(rng, 32),  // will trigger error since not hash of value
		Value: value,
	}
	responseErrs4 <- nil
	err = sb4.begin(lc4, sub, received, errs, end)
	assert.NotNil(t, err)
}

func TestDedup(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value1 := api.NewTestPublication(rng)
	value2 := api.NewTestPublication(rng)
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	key2, err := api.GetKey(value2)
	assert.Nil(t, err)
	fromPub1 := api.RandBytes(rng, api.ECPubKeyLength)
	fromPub2 := api.RandBytes(rng, api.ECPubKeyLength)

	slack := 1
	rp, err := NewRecentPublications(2)
	assert.Nil(t, err)
	newPVRs := make(chan *KeyedPub, slack)
	receivedPVRs := make(chan *pubValueReceipt, slack)
	toImpl := &to{
		new:    newPVRs,
		recent: rp,
	}

	go toImpl.dedup(receivedPVRs)

	// new
	pvr1in, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub1)
	assert.Nil(t, err)
	receivedPVRs <- pvr1in
	pv1out := <-newPVRs
	assert.Equal(t, pvr1in.pub, pv1out)

	// not new
	pvr2in, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub2)
	assert.Nil(t, err)
	receivedPVRs <- pvr2in
	var pv2out *KeyedPub
	select {
	case pv2out = <-newPVRs:
	default:
	}
	assert.Nil(t, pv2out)

	// new
	pvr3in, err := newPublicationValueReceipt(key2.Bytes(), value2, fromPub1)
	assert.Nil(t, err)
	receivedPVRs <- pvr3in
	pv3out := <-newPVRs
	assert.Equal(t, pvr3in.pub, pv3out)
}

func TestMonitorRunningErrorCount(t *testing.T) {
	errs := make(chan error, 8)
	fatal := make(chan error)
	maxRunningErrRate := float32(0.1)
	maxRunningErrCount := int(float32(maxRunningErrRate) * errQueueSize)

	go monitorRunningErrorCount(errs, fatal, maxRunningErrRate)

	// check get fatal error when go over threshold
	for c := 0; c < maxRunningErrCount; c++ {
		errs <- errors.New("some To error")
	}
	fataErr := <-fatal
	assert.Equal(t, ErrTooManySubscriptionErrs, fataErr)

	go monitorRunningErrorCount(errs, fatal, maxRunningErrRate)

	// check don't get fatal error when below threshold
	for c := 0; c < 200; c++ {
		var err error
		if c%25 == 0 {
			err = errors.New("some To error")
		}
		errs <- err
	}

	var fatalErr error
	select {
	case fatalErr = <-fatal:
	default:
	}
	assert.Nil(t, fatalErr)
}

type fixedSubscriptionBeginner struct {
	received chan *pubValueReceipt
	errs chan error
	subscribeErr error
}

func (f *fixedSubscriptionBeginner) begin(lc api.Subscriber, sub *api.Subscription,
	received chan *pubValueReceipt, errs chan error, end chan struct{}) error {
	if f.subscribeErr == nil {
		prv := <-f.received
		err := <-f.errs
		received <- prv
		errs <- err
	}
	return f.subscribeErr
}

type fixedClientBalancer struct {
	err error
}

func (f *fixedClientBalancer) Next() (api.LibrarianClient, error) {
	return nil, f.err
}

func (f *fixedClientBalancer) CloseAll() error {
	return nil
}

type fixedSubscriber struct {
	client api.Librarian_SubscribeClient
	err    error
}

func (f *fixedSubscriber) Subscribe(ctx context.Context, in *api.SubscribeRequest,
	opts ...grpc.CallOption) (api.Librarian_SubscribeClient, error) {
	return f.client, f.err
}

type fixedSigner struct {
	signature string
	err       error
}

func (f *fixedSigner) Sign(m proto.Message) (string, error) {
	return f.signature, f.err
}

type fixedLibrarianSubscribeClient struct {
	responses chan *api.SubscribeResponse
	err       chan error
}

func (f *fixedLibrarianSubscribeClient) Recv() (*api.SubscribeResponse, error) {
	sr := <-f.responses
	err := <-f.err
	return sr, err
}

// the stubs below just satisfy the Librarian_SubscribeClient interface
func (f *fixedLibrarianSubscribeClient) Header() (metadata.MD, error) {
	return nil, nil
}

func (f *fixedLibrarianSubscribeClient) Trailer() metadata.MD {
	return nil
}

func (f *fixedLibrarianSubscribeClient) CloseSend() error {
	return nil
}

func (f *fixedLibrarianSubscribeClient) Context() context.Context {
	return nil
}

func (f *fixedLibrarianSubscribeClient) SendMsg(m interface{}) error {
	return nil
}

func (f *fixedLibrarianSubscribeClient) RecvMsg(m interface{}) error {
	return nil
}
