package publish

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"google.golang.org/grpc"
	"golang.org/x/net/context"
	"math/rand"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/stretchr/testify/assert"
	"errors"
)

func TestAcquirer_Acquire_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := NewDefaultParameters()
	expectedDoc, docKey := api.NewTestDocument(rng)
	lc := &fixedGetter{
		responseValue: expectedDoc,
	}
	acq := NewAcquirer(clientID, signer, params)

	actualDoc, err := acq.Acquire(docKey, api.GetAuthorPub(expectedDoc), lc)
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
	lc := &fixedGetter{}

	// check that error from client.NewSignedTimeoutContext error bubbles up
	signer1 := &fixedSigner{ // causes client.NewSignedTimeoutContext to error
		signature: "",
		err: errors.New("some Sign error"),
	}
	acq1 := NewAcquirer(clientID, signer1, params)
	actualDoc, err := acq1.Acquire(docKey, api.GetAuthorPub(expectedDoc), lc)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)


	lc2 := &fixedGetter{
		err: errors.New("some Get error"),
	}
	acq2 := NewAcquirer(clientID, signer, params)
	actualDoc, err = acq2.Acquire(docKey, api.GetAuthorPub(expectedDoc), lc2)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)

	// check that different request ID causes error
	lc3 := &diffRequestIDGetter{rng}
	acq3 := NewAcquirer(clientID, signer, params)
	actualDoc, err = acq3.Acquire(docKey, api.GetAuthorPub(expectedDoc), lc3)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)

	// check that different author pub key creates error
	lc4 := &fixedGetter{
		responseValue: expectedDoc,
	}
	acq4 := NewAcquirer(clientID, signer, params)
	diffAuthorPub := ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))
	actualDoc, err = acq4.Acquire(docKey, diffAuthorPub, lc4)
	assert.NotNil(t, err)
	assert.Nil(t, actualDoc)
}

type fixedGetter struct {
	request *api.GetRequest
	responseValue *api.Document
	err error
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

