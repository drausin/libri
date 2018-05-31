package server

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestNewIDFromPublicKeyBytes_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	i1 := ecid.NewPseudoRandom(rng)
	i2, err := newIDFromPublicKeyBytes(i1.PublicKeyBytes())

	assert.Nil(t, err)
	assert.Equal(t, i1.ID(), i2)
}

func TestNewIDFromPublicKeyBytes_err(t *testing.T) {
	i, err := newIDFromPublicKeyBytes([]byte("not a pub key"))
	assert.NotNil(t, err)
	assert.Nil(t, i)
}

func TestCheckRequest_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{rqv: &alwaysRequestVerifier{}}
	selfID := ecid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, id.NewPseudoRandom(rng))
	requesterID, err := l.checkRequest(context.TODO(), rq, rq.Metadata)

	assert.Nil(t, err)
	assert.Equal(t, selfID.ID(), requesterID)
}

type neverRequestVerifier struct{}

func (rv *neverRequestVerifier) Verify(ctx context.Context, msg proto.Message,
	meta *api.RequestMetadata) error {
	return errors.New("some verification error")
}

func TestCheckRequest_newIDErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, id.NewPseudoRandom(rng))
	rq.Metadata.PubKey = []byte("bad pub key")
	l := &Librarian{}
	requesterID, err := l.checkRequest(context.TODO(), rq, rq.Metadata)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequest_verifyErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, id.NewPseudoRandom(rng))
	p, d := &fixedPreferer{}, &fixedDoctor{}
	l := &Librarian{
		rqv: &neverRequestVerifier{},
		rt:  routing.NewEmpty(selfID.ID(), p, d, routing.NewDefaultParameters()),
	}
	requesterID, err := l.checkRequest(context.TODO(), rq, rq.Metadata)

	assert.Zero(t, selfID.ID().Cmp(requesterID))
	assert.NotNil(t, err)
}

func TestCheckRequestAndKey_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	p, d := &fixedPreferer{}, &fixedDoctor{}
	l := &Librarian{
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rt:  routing.NewEmpty(selfID.ID(), p, d, routing.NewDefaultParameters()),
	}
	rq := client.NewGetRequest(selfID, key)
	requesterID, err := l.checkRequestAndKey(context.TODO(), rq, rq.Metadata, key.Bytes())

	assert.Nil(t, err)
	assert.Equal(t, selfID.ID(), requesterID)
}

func TestCheckRequestAndKey_checkRequestErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, key)
	rq.Metadata.PubKey = []byte("bad pub key")
	l := &Librarian{}
	requesterID, err := l.checkRequestAndKey(context.TODO(), rq, rq.Metadata, key.Bytes())

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKey_checkErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, key)
	p, d := &fixedPreferer{}, &fixedDoctor{}
	l := &Librarian{
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rt:  routing.NewEmpty(selfID.ID(), p, d, routing.NewDefaultParameters()),
	}
	requesterID, err := l.checkRequestAndKey(context.TODO(), rq, rq.Metadata, []byte("bad key"))

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKeyValue_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)
	p, d := &fixedPreferer{}, &fixedDoctor{}
	l := &Librarian{
		rt:  routing.NewEmpty(selfID.ID(), p, d, routing.NewDefaultParameters()),
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc: storage.NewHashKeyValueChecker(),
	}
	rq := client.NewGetRequest(selfID, key)
	requesterID, err := l.checkRequestAndKeyValue(context.TODO(), rq, rq.Metadata, key.Bytes(),
		value)

	assert.Nil(t, err)
	assert.Equal(t, selfID.ID(), requesterID)
}

func TestCheckRequestAndKeyValue_checkRequestErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)
	rq := client.NewGetRequest(selfID, key)
	rq.Metadata.PubKey = []byte("bad pub key")
	l := &Librarian{}
	requesterID, err := l.checkRequestAndKeyValue(context.TODO(), rq, rq.Metadata, key.Bytes(),
		value)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKeyValue_checkErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	value, _ := api.NewTestDocument(rng)
	key := id.NewPseudoRandom(rng) // bad key, not hash of value
	rq := client.NewGetRequest(selfID, key)
	p, d := &fixedPreferer{}, &fixedDoctor{}
	l := &Librarian{
		rt:  routing.NewEmpty(selfID.ID(), p, d, routing.NewDefaultParameters()),
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc: storage.NewHashKeyValueChecker(),
	}
	requesterID, err := l.checkRequestAndKeyValue(context.TODO(), rq, rq.Metadata, key.Bytes(),
		value)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

type fixedPreferer struct {
	prefer bool
}

func (f *fixedPreferer) Prefer(peerID1, peerID2 id.ID) bool {
	return f.prefer
}

type fixedDoctor struct {
	healthy bool
}

func (f *fixedDoctor) Healthy(peerID id.ID) bool {
	return f.healthy
}
