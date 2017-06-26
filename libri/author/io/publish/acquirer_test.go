package publish

import (
	"errors"
	"math/rand"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestAcquirer_Acquire_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := NewDefaultParameters()
	expectedDoc, docKey := api.NewTestDocument(rng)
	authorPub := api.GetAuthorPub(expectedDoc)
	lc := &fixedGetter{
		responseValue: expectedDoc,
	}
	acq := NewAcquirer(clientID, signer, params)

	actualDoc, err := acq.Acquire(docKey, authorPub, lc)
	assert.Nil(t, err)
	assert.Equal(t, actualDoc, expectedDoc)
	assert.Equal(t, docKey.Bytes(), lc.request.Key)

	actualDoc, err = acq.Acquire(docKey, authorPub, lc)
	assert.Nil(t, err)
	assert.Equal(t, actualDoc, expectedDoc)
	assert.Equal(t, docKey.Bytes(), lc.request.Key)
}

func TestAcquirer_Acquire_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := NewDefaultParameters()
	expectedDoc, docKey := api.NewTestDocument(rng)
	authorPub := api.GetAuthorPub(expectedDoc)
	lc := &fixedGetter{}

	// check that error from client.NewSignedTimeoutContext error bubbles up
	signer1 := &fixedSigner{ // causes client.NewSignedTimeoutContext to error
		signature: "",
		err:       errors.New("some Sign error"),
	}
	acq1 := NewAcquirer(clientID, signer1, params)
	actualDoc, err := acq1.Acquire(docKey, authorPub, lc)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)

	lc2 := &fixedGetter{
		err: errors.New("some Get error"),
	}
	acq2 := NewAcquirer(clientID, signer, params)
	actualDoc, err = acq2.Acquire(docKey, authorPub, lc2)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)

	// check that different request ID causes error
	lc3 := &diffRequestIDGetter{rng}
	acq3 := NewAcquirer(clientID, signer, params)
	actualDoc, err = acq3.Acquire(docKey, authorPub, lc3)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)

	lc4 := &fixedGetter{
		responseValue: expectedDoc,
	}
	acq4 := NewAcquirer(clientID, signer, params)
	diffAuthorPub := ecid.NewPseudoRandom(rng).PublicKeyBytes()
	actualDoc, err = acq4.Acquire(docKey, diffAuthorPub, lc4)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)
}

func TestSingleStoreAcquirer_Acquire_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	doc, docKey := api.NewTestDocument(rng)
	authorPub := api.GetAuthorPub(doc)
	storer := &fixedStorer{}
	acq := NewSingleStoreAcquirer(
		&fixedAcquirer{doc: doc},
		storer,
	)
	err := acq.Acquire(docKey, authorPub, &fixedGetter{})
	assert.Nil(t, err)
	assert.Equal(t, docKey, storer.storedKey)
	assert.Equal(t, doc, storer.storedValue)
}

func TestSingleStoreAcquirer_Acquire_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	doc, docKey := api.NewTestDocument(rng)
	authorPub := api.GetAuthorPub(doc)
	lc := &fixedGetter{}

	// check inner acquire error bubbles up
	acq1 := NewSingleStoreAcquirer(
		&fixedAcquirer{err: errors.New("some Acquire error")},
		&fixedStorer{},
	)
	err := acq1.Acquire(docKey, authorPub, lc)
	assert.NotNil(t, err)

	// check store error bubbles up
	acq2 := NewSingleStoreAcquirer(
		&fixedAcquirer{},
		&fixedStorer{err: errors.New("some Store error")},
	)
	err = acq2.Acquire(docKey, authorPub, lc)
	assert.NotNil(t, err)
}

func TestMultiStoreAcquirer_Acquire_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}
	for _, nDocs := range []int{1, 2, 4, 8, 16} {
		docKeys := make([]id.ID, nDocs)
		for i := 0; i < nDocs; i++ {
			docKeys[i] = id.NewPseudoRandom(rng)
		}
		authorKey := ecid.NewPseudoRandom(rng).PublicKeyBytes()
		for _, getParallelism := range []uint32{1, 2, 3} {
			slAcq := &fixedSingleStoreAcquirer{
				acquiredKeys: make(map[string]struct{}),
			}
			params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout,
				DefaultPutParallelism, getParallelism)
			assert.Nil(t, err)
			msAcq := NewMultiStoreAcquirer(slAcq, params)

			err = msAcq.Acquire(docKeys, authorKey, cb)
			assert.Nil(t, err)

			// check all keys have been "acquired"
			for _, docKey := range docKeys {
				_, in := slAcq.acquiredKeys[docKey.String()]
				assert.True(t, in)
			}
		}
	}
}

func TestMultiStoreAcquirer_Acquire_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}
	for _, nDocs := range []int{1, 2, 4, 8, 16} {
		docKeys := make([]id.ID, nDocs)
		for i := 0; i < nDocs; i++ {
			docKeys[i] = id.NewPseudoRandom(rng)
		}
		authorKey := ecid.NewPseudoRandom(rng).PublicKeyBytes()
		for _, getParallelism := range []uint32{1, 2, 3} {
			slAcq := &fixedSingleStoreAcquirer{
				err: errors.New("some Acquire error"),
			}
			params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout,
				DefaultPutParallelism, getParallelism)
			assert.Nil(t, err)
			mlAcq := NewMultiStoreAcquirer(slAcq, params)

			err = mlAcq.Acquire(docKeys, authorKey, cb)
			assert.NotNil(t, err)
		}
	}
}

type fixedGetter struct {
	request       *api.GetRequest
	responseValue *api.Document
	err           error
}

func (f *fixedGetter) Get(ctx context.Context, in *api.GetRequest, opts ...grpc.CallOption) (
	*api.GetResponse, error) {

	f.request = in
	return &api.GetResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: in.Metadata.RequestId,
		},
		Value: f.responseValue,
	}, f.err
}

type diffRequestIDGetter struct {
	rng *rand.Rand
}

func (p *diffRequestIDGetter) Get(
	ctx context.Context, in *api.GetRequest, opts ...grpc.CallOption,
) (*api.GetResponse, error) {

	return &api.GetResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: api.RandBytes(p.rng, 32),
		},
	}, nil
}

type fixedAcquirer struct {
	doc *api.Document
	err error
}

func (f *fixedAcquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (
	*api.Document, error) {
	return f.doc, f.err
}

type fixedStorer struct {
	err         error
	storedKey   id.ID
	storedValue *api.Document
}

func (f *fixedStorer) Store(key id.ID, value *api.Document) error {
	f.storedKey, f.storedValue = key, value
	return f.err
}

type fixedSingleStoreAcquirer struct {
	err          error
	mu           sync.Mutex
	acquiredKeys map[string]struct{}
}

func (f *fixedSingleStoreAcquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.acquiredKeys[docKey.String()] = struct{}{}
	}
	return f.err
}
