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
)

func TestLoadPublisher_Publish_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := &Parameters{
		PutTimeout: DefaultPutTimeout,
		PutParallelism: DefaultPutParallelism,
	}
	lc := &fixedPutter{
		err: nil,
	}
	pub := NewPublisher(clientID, signer, params)

	doc, expectedDocKey := api.NewTestDocument(rng)
	actualDocKey, err := pub.Publish(doc, lc)
	assert.Nil(t, err)
	assert.Equal(t, expectedDocKey, actualDocKey)
	assert.Equal(t, doc, lc.request.Value)
}

func TestLoadPublisher_Publish_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	clientID := ecid.NewPseudoRandom(rng)
	signer := client.NewSigner(clientID.Key())
	params := &Parameters{
		PutTimeout: DefaultPutTimeout,
		PutParallelism: DefaultPutParallelism,
	}
	lc := &fixedPutter{
		err: nil,
	}
	doc, _ := api.NewTestDocument(rng)

	pub1 := NewPublisher(clientID, signer, params)

	// check that error from bad document bubbles up
	docKey, err := pub1.Publish(nil, lc)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)


	signer2 := &fixedSigner{ // causes client.NewSignedTimeoutContext to error
		signature: "",
		err: errors.New("some Sign error"),
	}
	pub2 := NewPublisher(clientID, signer2, params)

	// check that error from client.NewSignedTimeoutContext error bubbles up
	docKey, err = pub2.Publish(doc, lc)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)


	lc3 := &fixedPutter{
		err: errors.New("some Put error"),
	}
	pub3 := NewPublisher(clientID, signer, params)

	// check that Put error bubbles up
	docKey, err = pub3.Publish(doc, lc3)
	assert.NotNil(t, err)
	assert.Nil(t, docKey)


	lc4 := &diffRequestIDPutter{rng}
	pub4 := NewPublisher(clientID, signer, params)

	// check that different request ID causes error
	docKey, err = pub4.Publish(doc, lc4)
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

	err := slPub.Publish(docKey, lc)
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


	slPub1 := NewSingleLoadPublisher(pub, &errDocLoader{})

	// check docL.Load error bubbles up
	err := slPub1.Publish(docKey, lc)
	assert.NotNil(t, err)


	slPub2 := NewSingleLoadPublisher(pub, docL)

	// check that missing doc triggers error
	err = slPub2.Publish(docKey, lc)
	assert.Equal(t, ErrUnexpectedMissingDocument, err)


	pub3 := &fixedPublisher{
		publishErr: errors.New("some Publish error"),
	}
	slPub3 := NewSingleLoadPublisher(pub3, docL)
	docL.docs[docKey.String()] = doc

	// check that missing doc triggers error
	err = slPub3.Publish(docKey, lc)
	assert.NotNil(t, err)
}

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

func (p *fixedPublisher) Publish(doc *api.Document, lc api.Putter) (id.ID, error) {
	p.doc = doc
	return p.publishID, p.publishErr
}

type fixedSingleLoadPublisher struct {
	err error
}

func (f *fixedSingleLoadPublisher) Publish(docKey id.ID, lc api.Putter) error {
	return f.err
}

