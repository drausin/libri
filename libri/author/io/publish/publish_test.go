package publish

import (
	"testing"
	"math/rand"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/drausin/libri/libri/common/id"
	"sync"
	"time"
)

func TestNewParameters_ok(t *testing.T) {
	params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout, DefaultPutParallelism,
		DefaultGetParallelism)
	assert.Nil(t, err)
	assert.NotNil(t, params)
}

func TestNewParameters_err(t *testing.T) {
	params, err := NewParameters(0 * time.Second, DefaultGetTimeout, DefaultPutParallelism,
		DefaultGetParallelism)
	assert.Equal(t, ErrPutTimeoutZeroValue, err)
	assert.Nil(t, params)

	params, err = NewParameters(
		DefaultPutTimeout,
		0 * time.Second,
		DefaultPutParallelism,
		DefaultGetParallelism,
	)
	assert.Equal(t, ErrGetTimeoutZeroValue, err)
	assert.Nil(t, params)

	params, err = NewParameters(DefaultPutTimeout, DefaultGetTimeout, 0, DefaultGetParallelism)
	assert.Equal(t, ErrPutParallelismZeroValue, err)
	assert.Nil(t, params)

	params, err = NewParameters(DefaultPutTimeout, DefaultGetTimeout, DefaultPutParallelism, 0)
	assert.Equal(t, ErrGetParallelismZeroValue, err)
	assert.Nil(t, params)
}

func TestPublisher_Publish_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := NewDefaultParameters()
	lc := &fixedPutter{
		err: nil,
	}
	pub := NewPublisher(clientID, signer, params)

	doc, expectedDocKey := api.NewTestDocument(rng)
	actualDocKey, err := pub.Publish(doc, api.GetAuthorPub(doc), lc)
	assert.Nil(t, err)
	assert.Equal(t, expectedDocKey, actualDocKey)
	assert.Equal(t, doc, lc.request.Value)
}

func TestPublisher_Publish_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := NewDefaultParameters()
	lc := &fixedPutter{}
	doc, _ := api.NewTestDocument(rng)

	pub := NewPublisher(clientID, signer, params)

	// check that error from bad document bubbles up
	diffAuthorPub := ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))
	docKey, err := pub.Publish(nil, diffAuthorPub, lc)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)

	// check that different author pub key creates error
	docKey, err = pub.Publish(doc, diffAuthorPub, lc)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)

	signer2 := &fixedSigner{ // causes client.NewSignedTimeoutContext to error
		signature: "",
		err: errors.New("some Sign error"),
	}
	pub = NewPublisher(clientID, signer2, params)

	// check that error from client.NewSignedTimeoutContext error bubbles up
	docKey, err = pub.Publish(doc, api.GetAuthorPub(doc), lc)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)


	lc3 := &fixedPutter{
		err: errors.New("some Put error"),
	}
	pub = NewPublisher(clientID, signer, params)

	// check that Put error bubbles up
	docKey, err = pub.Publish(doc, api.GetAuthorPub(doc), lc3)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)


	lc4 := &diffRequestIDPutter{rng}
	pub = NewPublisher(clientID, signer, params)

	// check that different request ID causes error
	docKey, err = pub.Publish(doc, api.GetAuthorPub(doc), lc4)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)
}

func TestSingleLoadPublisher_Publish_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	pub := &fixedPublisher{}
	docL := &memDocLoader{
		docs: make(map[string]*api.Document),
	}
	lc := &fixedPutter{}
	slPub := NewSingleLoadPublisher(pub, docL)
	doc, docKey := api.NewTestDocument(rng)

	// add document to memDocLoader so that it's present for docL.Load()
	docL.docs[docKey.String()] = doc

	err := slPub.Publish(docKey, api.GetAuthorPub(doc), lc)
	assert.Nil(t, err)
}

func TestSingleLoadPublisher_Publish_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	pub := &fixedPublisher{}
	docL := &memDocLoader{
		docs: make(map[string]*api.Document),
	}
	lc := &fixedPutter{}
	doc, docKey := api.NewTestDocument(rng)

	// check docL.Load error bubbles up
	slPub := NewSingleLoadPublisher(pub, &errDocLoader{})
	err := slPub.Publish(docKey, api.GetAuthorPub(doc), lc)
	assert.NotNil(t, err)

	// check that missing doc triggers error
	slPub = NewSingleLoadPublisher(pub, docL)
	err = slPub.Publish(docKey, api.GetAuthorPub(doc), lc)
	assert.Equal(t, ErrUnexpectedMissingDocument, err)

	pub3 := &fixedPublisher{
		publishErr: errors.New("some Publish error"),
	}
	slPub = NewSingleLoadPublisher(pub3, docL)
	docL.docs[docKey.String()] = doc

	// check that missing doc triggers error
	err = slPub.Publish(docKey, api.GetAuthorPub(doc), lc)
	assert.NotNil(t, err)
}

func TestMultiLoadPublisher_Publish_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &nilClientBalancer{}
	for _, nDocs := range []int{1, 2, 4, 8, 16} {
		docKeys := make([]id.ID, nDocs)
		for i := 0; i < nDocs; i++ {
			docKeys[i] = id.NewPseudoRandom(rng)
		}
		authorKey := ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))
		for _, putParallelism := range []uint32{1, 2, 3} {
			slPub := &fixedSingleLoadPublisher{
				publishedKeys: make(map[string]struct{}),
			}
			params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout,
				putParallelism, DefaultGetParallelism)
			assert.Nil(t, err)
			mlPub := NewMultiLoadPublisher(slPub, params)

			err = mlPub.Publish(docKeys, authorKey, cb)
			assert.Nil(t, err)

			// check all keys have been "published"
			for _, docKey := range docKeys {
				_, in := slPub.publishedKeys[docKey.String()]
				assert.True(t, in)
			}
		}
	}
}

func TestMultiLoadPublisher_Publish_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &nilClientBalancer{}
	for _, nDocs := range []int{1, 2, 4, 8, 16} {
		docKeys := make([]id.ID, nDocs)
		for i := 0; i < nDocs; i++ {
			docKeys[i] = id.NewPseudoRandom(rng)
		}
		authorKey := ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))
		for _, putParallelism := range []uint32{1, 2, 3} {
			slPub := &fixedSingleLoadPublisher{
				err: errors.New("some Publish error"),
			}
			params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout,
				putParallelism, DefaultGetParallelism)
			assert.Nil(t, err)
			mlPub := NewMultiLoadPublisher(slPub, params)

			err = mlPub.Publish(docKeys, authorKey, cb)
			assert.NotNil(t, err)
		}
	}
}

// TODO TestMultiAcquirePublish

type fixedPutter struct {
	request *api.PutRequest
	err error
}

func (p *fixedPutter) Put(
	ctx context.Context, in *api.PutRequest, opts ...grpc.CallOption,
) (*api.PutResponse, error) {

	p.request = in
	return &api.PutResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: in.Metadata.RequestId,
		},
	}, p.err
}

type diffRequestIDPutter struct {
	rng *rand.Rand
}

func (p *diffRequestIDPutter) Put(
	ctx context.Context, in *api.PutRequest, opts ...grpc.CallOption,
) (*api.PutResponse, error) {

	return &api.PutResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: api.RandBytes(p.rng, 32),
		},
	}, nil
}

type fixedSigner struct {
	signature string
	err error
}

func (f *fixedSigner) Sign(m proto.Message) (string, error) {
	return f.signature, f.err
}

type memDocLoader struct {
	docs map[string]*api.Document
}

func (m *memDocLoader) Load(key id.ID) (*api.Document, error) {
	value, _ := m.docs[key.String()]
	return value, nil
}

type errDocLoader struct {}

func (m *errDocLoader) Load(key id.ID) (*api.Document, error) {
	return nil, errors.New("some Load error")
}

type fixedPublisher struct {
	doc *api.Document
	publishID id.ID
	publishErr error
}

func (p *fixedPublisher) Publish(doc *api.Document, authorPub []byte, lc api.Putter) (
	id.ID, error) {
	p.doc = doc
	return p.publishID, p.publishErr
}

type fixedSingleLoadPublisher struct {
	mu sync.Mutex
	publishedKeys map[string]struct{}
	err error
}

func (f *fixedSingleLoadPublisher) Publish(docKey id.ID, authorPub []byte, lc api.Putter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.publishedKeys[docKey.String()] = struct{}{}
	}
	return f.err
}

type nilClientBalancer struct {}

func (*nilClientBalancer) Next() (api.LibrarianClient, error) {
	return nil, nil
}

func (*nilClientBalancer) CloseAll() error {
	return nil
}

