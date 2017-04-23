package publish

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/golang/protobuf/proto"
	"errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestNewParameters_ok(t *testing.T) {
	params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout, DefaultPutParallelism,
		DefaultGetParallelism)
	assert.Nil(t, err)
	assert.NotNil(t, params)
}

func TestNewParameters_err(t *testing.T) {
	params, err := NewParameters(0*time.Second, DefaultGetTimeout, DefaultPutParallelism,
		DefaultGetParallelism)
	assert.Equal(t, ErrPutTimeoutZeroValue, err)
	assert.Nil(t, params)

	params, err = NewParameters(
		DefaultPutTimeout,
		0*time.Second,
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
		err:       errorsNew("some Sign error"),
	}
	pub = NewPublisher(clientID, signer2, params)

	// check that error from client.NewSignedTimeoutContext error bubbles up
	docKey, err = pub.Publish(doc, api.GetAuthorPub(doc), lc)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)

	lc3 := &fixedPutter{
		err: errorsNew("some Put error"),
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
	docL := &memDocStorerLoader{
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
	docL := &memDocStorerLoader{
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
		publishErr: errorsNew("some Publish error"),
	}
	slPub = NewSingleLoadPublisher(pub3, docL)
	docL.docs[docKey.String()] = doc

	// check that missing doc triggers error
	err = slPub.Publish(docKey, api.GetAuthorPub(doc), lc)
	assert.NotNil(t, err)
}

func TestMultiLoadPublisher_Publish_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}
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
	cb := &fixedClientBalancer{}
	for _, nDocs := range []int{1, 2, 4, 8, 16} {
		docKeys := make([]id.ID, nDocs)
		for i := 0; i < nDocs; i++ {
			docKeys[i] = id.NewPseudoRandom(rng)
		}
		authorKey := ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))
		for _, putParallelism := range []uint32{1, 2, 3} {
			slPub := &fixedSingleLoadPublisher{
				err: errorsNew("some Publish error"),
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

func TestMultiAcquirePublish(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}

	getParallelisms := []uint32{1, 2, 3}
	putParallelisms := []uint32{1, 2, 3}
	numDocs := []uint32{1, 2, 4, 8, 16}

	for _, c := range caseCrossProduct(getParallelisms, putParallelisms, numDocs) {

		// setup
		params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout,
			c.putParallelism, c.getParallelism)
		assert.Nil(t, err)
		pubAcq := &memPublisherAcquirer{
			docs: make(map[string]*api.Document),
		}
		docSL1 := &memDocStorerLoader{
			docs: make(map[string]*api.Document),
		}
		mlP := NewMultiLoadPublisher(
			NewSingleLoadPublisher(pubAcq, docSL1),
			params,
		)
		docSL2 := &memDocStorerLoader{
			docs: make(map[string]*api.Document),
		}
		msA := NewMultiStoreAcquirer(
			NewSingleStoreAcquirer(pubAcq, docSL2),
			params,
		)
		docs := make([]*api.Document, c.numDocs)
		docKeys := make([]id.ID, c.numDocs)
		for i := uint32(0); i < c.numDocs; i++ {
			docs[i], docKeys[i] = api.NewTestDocument(rng)

			// load first SL with documents for publiser
			err = docSL1.Store(docKeys[i], docs[i])
			assert.Nil(t, err)
		}

		// publish & then acquire docs
		err = mlP.Publish(docKeys, nil, cb)
		assert.Nil(t, err)
		err = msA.Acquire(docKeys, nil, cb)
		assert.Nil(t, err)

		// test that states of both DocumentStorerLoaders contain all the docs
		assert.Equal(t, int(c.numDocs), len(docSL1.docs))
		for i := uint32(0); i < c.numDocs; i++ {
			storedDoc, in := docSL1.docs[docKeys[i].String()]
			assert.True(t, in)
			assert.Equal(t, docs[i], storedDoc)
		}
		assert.Equal(t, docSL1, docSL2)
	}
}

type fixedPutter struct {
	request *api.PutRequest
	err     error
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

type memPutterGetter struct {
	storage map[string]*api.Document
}

func (p *memPutterGetter) Put(
	ctx context.Context, in *api.PutRequest, opts ...grpc.CallOption,
) (*api.PutResponse, error) {

	p.storage[id.FromBytes(in.Key).String()] = in.Value
	return &api.PutResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: in.Metadata.RequestId,
		},
	}, nil
}

func (p *memPutterGetter) Get(ctx context.Context, in *api.GetRequest, opts ...grpc.CallOption) (
	*api.GetResponse, error) {

	return &api.GetResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: in.Metadata.RequestId,
		},
		Value: p.storage[id.FromBytes(in.Key).String()],
	}, nil
}

type fixedSigner struct {
	signature string
	err       error
}

func (f *fixedSigner) Sign(m proto.Message) (string, error) {
	return f.signature, f.err
}

type memDocStorerLoader struct {
	docs map[string]*api.Document
	mu   sync.Mutex
}

func (m *memDocStorerLoader) Load(key id.ID) (*api.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, _ := m.docs[key.String()]
	return value, nil
}

func (m *memDocStorerLoader) Store(key id.ID, value *api.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[key.String()] = value
	return nil
}

type errDocLoader struct{}

func (m *errDocLoader) Load(key id.ID) (*api.Document, error) {
	return nil, errorsNew("some Load error")
}

type fixedPublisher struct {
	doc        *api.Document
	publishID  id.ID
	publishErr error
}

func (p *fixedPublisher) Publish(doc *api.Document, authorPub []byte, lc api.Putter) (
	id.ID, error) {
	p.doc = doc
	return p.publishID, p.publishErr
}

type memPublisherAcquirer struct {
	docs map[string]*api.Document
	mu   sync.Mutex
}

func (p *memPublisherAcquirer) Publish(doc *api.Document, authorPub []byte, lc api.Putter) (
	id.ID, error) {
	docKey, err := api.GetKey(doc)
	if err != nil {
		panic(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.docs[docKey.String()] = doc
	return docKey, nil
}

func (p *memPublisherAcquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (
	*api.Document, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.docs[docKey.String()], nil
}

type fixedSingleLoadPublisher struct {
	mu            sync.Mutex
	publishedKeys map[string]struct{}
	err           error
}

func (f *fixedSingleLoadPublisher) Publish(docKey id.ID, authorPub []byte, lc api.Putter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.publishedKeys[docKey.String()] = struct{}{}
	}
	return f.err
}

type fixedClientBalancer struct {
	client api.LibrarianClient
	err    error
}

func (f *fixedClientBalancer) Next() (api.LibrarianClient, error) {
	return f.client, f.err
}

func (f *fixedClientBalancer) CloseAll() error {
	return f.err
}

type publishTestCase struct {
	getParallelism uint32
	putParallelism uint32
	numDocs        uint32
}

func caseCrossProduct(
	getParallelisms []uint32, putParallelisms []uint32, numDocss []uint32,
) []*publishTestCase {
	cases := make([]*publishTestCase, 0)
	for _, getParallelism := range getParallelisms {
		for _, putParallelism := range putParallelisms {
			for _, numDocs := range numDocss {
				cases = append(cases, &publishTestCase{
					getParallelism: getParallelism,
					putParallelism: putParallelism,
					numDocs:        numDocs,
				})
			}
		}
	}
	return cases
}
