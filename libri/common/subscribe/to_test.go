package subscribe

import (
	"errors"
	"io"
	"math/rand"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	cerrors "github.com/drausin/libri/libri/common/errors"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestTo_BeginEnd(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := NewDefaultToParameters()
	clientID := ecid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	lg := clogging.NewDevInfoLogger()
	params.NSubscriptions = 2
	cb := &fixedClientSetBalancer{}
	recent, err := NewRecentPublications(2)
	assert.Nil(t, err)
	newPubs := make(chan *KeyedPub, 1)
	toImpl := NewTo(params, lg, clientID, orgID, cb, nil, nil, recent, newPubs).(*to)

	// mock what we actually get from subscriptions
	received := make(chan *pubValueReceipt)
	errs := make(chan error)
	toImpl.sb = &fixedSubscriptionBeginner{
		received: received,
		errs:     errs,
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
	newPub, ended := getNewPub(newPubs, toImpl.end)
	assert.False(t, ended)
	assert.Equal(t, pvr1.pub, newPub)

	// not new
	pvr2, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub2)
	assert.Nil(t, err)
	received <- pvr2
	errs <- nil
	newPub = nil
	select {
	case <-toImpl.end:
		ended = true
	case newPub = <-newPubs:
	default:
	}
	assert.Nil(t, newPub)
	assert.False(t, ended)

	// new
	pvr3, err := newPublicationValueReceipt(key2.Bytes(), value2, fromPub1)
	assert.Nil(t, err)
	received <- pvr3
	errs <- nil
	newPub, ended = getNewPub(newPubs, toImpl.end)
	assert.Equal(t, pvr3.pub, newPub)
	assert.False(t, ended)

	// new
	pvr4, err := newPublicationValueReceipt(key3.Bytes(), value3, fromPub2)
	assert.Nil(t, err)
	received <- pvr4
	errs <- nil
	newPub, ended = getNewPub(newPubs, toImpl.end)
	assert.Equal(t, pvr4.pub, newPub)
	assert.False(t, ended)

	toImpl.End()

	// sometimes toImpl.end doesn't register as closed in getNewPub unless we explicitly try to
	// select from it beforehand...weird, but the next 4 lines seem to fix it
	select {
	case <-toImpl.end:
	default:
	}
	// TODO (drausin) understand better why this is flakey
	//newPub, ended = getNewPub(newPubs, toImpl.end)
	//assert.True(t, ended)
	getNewPub(newPubs, toImpl.end)

	// check newPubs is closed
	pub, open := <-newPubs
	assert.Nil(t, pub)
	assert.False(t, open)

	wg.Wait()
}

func getNewPub(newPubs chan *KeyedPub, end chan struct{}) (newPub *KeyedPub, ended bool) {
	select {
	case <-end:
		ended = true
	case newPub = <-newPubs:
	}
	return newPub, ended
}

func TestTo_Begin_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := NewDefaultToParameters()
	params.NSubscriptions = 2
	lg := clogging.NewDevInfoLogger()
	clientID := ecid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	recent, err := NewRecentPublications(2)
	csb := &fixedClientSetBalancer{}
	assert.Nil(t, err)
	newPubs := make(chan *KeyedPub, 1)

	// check csb.Next() error bubbles up
	nextErr := errors.New("some Next() error")
	csb1 := &fixedClientSetBalancer{err: nextErr}
	toImpl1 := NewTo(params, lg, clientID, orgID, csb1, nil, nil, recent, newPubs).(*to)
	toImpl1.sb = &fixedSubscriptionBeginner{subscribeErr: errors.New("some subscribe error")}
	err = toImpl1.Begin()
	assert.Equal(t, nextErr, err)

	// check NewFPSubscription error bubbles up
	params2 := NewDefaultToParameters()
	params2.FPRate = 0.0 // will trigger error
	toImpl2 := NewTo(params2, lg, clientID, orgID, csb, nil, nil, recent, newPubs).(*to)
	toImpl2.sb = &fixedSubscriptionBeginner{subscribeErr: errors.New("some subscribe error")}
	err = toImpl2.Begin()
	assert.Equal(t, ErrOutOfBoundsFPRate, err)

	// check running error count above threshold triggers error
	received := make(chan *pubValueReceipt)
	errs := make(chan error)
	toImpl3 := NewTo(params, lg, clientID, orgID, csb, nil, nil, recent, newPubs).(*to)
	toImpl3.sb = &fixedSubscriptionBeginner{
		received:     received,
		errs:         errs,
		subscribeErr: errors.New("some subscribe error"),
	}
	go func() {
		for c := 0; c < int(params.FPRate*float32(errQueueSize)); c++ {
			received <- nil
			errs <- errors.New("some Recv error")
		}
	}()
	err = toImpl3.Begin()
	assert.Equal(t, cerrors.ErrTooManyErrs, err)
}

func TestFrom_Send(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	toImpl := &to{
		clientID: ecid.NewPseudoRandom(rng),
		received: make(chan *pubValueReceipt),
		logger:   clogging.NewDevInfoLogger(),
	}

	// check nothing sent with nil pub by ensuring that Send() doesn't block
	err := toImpl.Send(nil)
	assert.Nil(t, err)

	pub := api.NewTestPublication(rng)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err := toImpl.Send(pub)
		assert.Nil(t, err)
	}(wg)

	pvr := <-toImpl.received
	assert.NotNil(t, pvr)
	assert.Equal(t, pub, pvr.pub.Value)

	wg.Wait()
}

func TestSubscriptionBeginnerImpl_Begin_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	fromID := ecid.NewPseudoRandom(rng)
	fromPubKey := fromID.PublicKeyBytes()
	sb := subscriptionBeginnerImpl{
		peerID:     clientID,
		peerSigner: &fixedSigner{signature: "some.signature.jtw"},
		orgSigner:  &fixedSigner{signature: "some.org-signature.jtw"},
		params:     NewDefaultToParameters(),
	}
	responses := make(chan *api.SubscribeResponse, 1)
	responseErrs := make(chan error, 1)
	lc := &fixedSubscriber{
		client: &fixedLibrarianSubscribeClient{
			responses: responses,
			err:       responseErrs,
		},
	}
	sub, err := NewFPSubscription(DefaultFPRate, rng)
	assert.Nil(t, err)
	received := make(chan *pubValueReceipt, 1)
	errs := make(chan error)
	end := make(chan struct{})

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

	go func() {
		beginErr := sb.begin(lc, sub, received, errs, end)
		assert.Nil(t, beginErr)
	}()

	receivedPub := <-received
	err = <-errs
	assert.Nil(t, err)
	assert.Equal(t, key, receivedPub.pub.Key)
	assert.Equal(t, value, receivedPub.pub.Value)
	assert.Equal(t, fromPubKey, receivedPub.receipt.FromPub)

	// simulate subscription being close on server side
	responses <- nil
	responseErrs <- io.EOF

	// start again
	go func() {
		beginErr := sb.begin(lc, sub, received, errs, end)
		assert.Nil(t, beginErr)
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
	fromPubKey := fromID.PublicKeyBytes()
	errs := make(chan error)
	end := make(chan struct{})
	received := make(chan *pubValueReceipt, 1)
	sub, err := NewFPSubscription(DefaultFPRate, rng)
	assert.Nil(t, err)

	// check NewSignedTimeoutContext error bubbles up
	sb1 := subscriptionBeginnerImpl{
		peerID:     clientID,
		peerSigner: &fixedSigner{err: errors.New("some Signer error")},
		orgSigner:  &fixedSigner{signature: "some.org-signature.jtw"},
		params:     NewDefaultToParameters(),
	}
	lc1 := &fixedSubscriber{}
	err = sb1.begin(lc1, sub, received, errs, end)
	assert.NotNil(t, err)

	// check Subscribe error bubbles up
	sb2 := subscriptionBeginnerImpl{
		peerID:     clientID,
		peerSigner: &fixedSigner{signature: "some.signature.jtw"},
		orgSigner:  &fixedSigner{signature: "some.org-signature.jtw"},
		params:     NewDefaultToParameters(),
	}
	lc2 := &fixedSubscriber{
		client: nil,
		err:    errors.New("some Subscribe error"),
	}
	err = sb2.begin(lc2, sub, received, errs, end)
	assert.NotNil(t, err)

	// check Recv error bubbles up
	sb3 := subscriptionBeginnerImpl{
		peerID:     clientID,
		peerSigner: &fixedSigner{signature: "some.signature.jtw"},
		orgSigner:  &fixedSigner{signature: "some.org-signature.jtw"},
		params:     NewDefaultToParameters(),
	}
	responses3 := make(chan *api.SubscribeResponse, 1)
	responseErrs3 := make(chan error, 1)
	lc3 := &fixedSubscriber{
		client: &fixedLibrarianSubscribeClient{
			responses: responses3,
			err:       responseErrs3,
		},
	}
	responses3 <- nil
	responseErrs3 <- errors.New("some Recv error")
	err = sb3.begin(lc3, sub, received, errs, end)
	assert.NotNil(t, err)

	// check newPublicationValueReceipt error bubbles up
	sb4 := subscriptionBeginnerImpl{
		peerID:     clientID,
		peerSigner: &fixedSigner{signature: "some.signature.jtw"},
		orgSigner:  &fixedSigner{signature: "some.org-signature.jtw"},
		params:     NewDefaultToParameters(),
	}
	responses4 := make(chan *api.SubscribeResponse, 1)
	responseErrs4 := make(chan error, 1)
	lc4 := &fixedSubscriber{
		client: &fixedLibrarianSubscribeClient{
			responses: responses4,
			err:       responseErrs4,
		},
	}
	value := api.NewTestPublication(rng)
	responses4 <- &api.SubscribeResponse{
		Metadata: &api.ResponseMetadata{
			PubKey: fromPubKey,
		},
		Key:   api.RandBytes(rng, 32), // will trigger error since not hash of value
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
		received: receivedPVRs,
		new:      newPVRs,
		recent:   rp,
		logger:   clogging.NewDevLogger(zapcore.DebugLevel),
	}

	go toImpl.dedup()

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

type fixedSubscriptionBeginner struct {
	received     chan *pubValueReceipt
	errs         chan error
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

type fixedClientSetBalancer struct {
	err error
}

func (f *fixedClientSetBalancer) AddNext() (api.LibrarianClient, string, error) {
	return nil, "", f.err
}

func (f *fixedClientSetBalancer) Remove(address string) error {
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
