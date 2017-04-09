package server

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestNewIDFromPublicKeyBytes_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	i1 := ecid.NewPseudoRandom(rng)
	i2, err := newIDFromPublicKeyBytes(ecid.ToPublicKeyBytes(i1))

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
	rq := client.NewGetRequest(selfID, cid.NewPseudoRandom(rng))
	requesterID, err := l.checkRequest(nil, rq, rq.Metadata)

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
	rq := client.NewGetRequest(selfID, cid.NewPseudoRandom(rng))
	rq.Metadata.PubKey = []byte("bad pub key")
	l := &Librarian{}
	requesterID, err := l.checkRequest(nil, rq, rq.Metadata)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequest_verifyErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, cid.NewPseudoRandom(rng))
	l := &Librarian{
		rqv: &neverRequestVerifier{},
		rt:  routing.NewEmpty(selfID, routing.NewDefaultParameters()),
	}
	requesterID, err := l.checkRequest(nil, rq, rq.Metadata)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKey_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	l := &Librarian{
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rt:  routing.NewEmpty(selfID, routing.NewDefaultParameters()),
	}
	rq := client.NewGetRequest(selfID, key)
	requesterID, err := l.checkRequestAndKey(nil, rq, rq.Metadata, key.Bytes())

	assert.Nil(t, err)
	assert.Equal(t, selfID.ID(), requesterID)
}

func TestCheckRequestAndKey_checkRequestErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, key)
	rq.Metadata.PubKey = []byte("bad pub key")
	l := &Librarian{}
	requesterID, err := l.checkRequestAndKey(nil, rq, rq.Metadata, key.Bytes())

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKey_checkErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(selfID, key)
	l := &Librarian{
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rt:  routing.NewEmpty(selfID, routing.NewDefaultParameters()),
	}
	requesterID, err := l.checkRequestAndKey(nil, rq, rq.Metadata, []byte("bad key"))

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKeyValue_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)
	l := &Librarian{
		rt:  routing.NewEmpty(selfID, routing.NewDefaultParameters()),
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc: storage.NewHashKeyValueChecker(),
	}
	rq := client.NewGetRequest(selfID, key)
	requesterID, err := l.checkRequestAndKeyValue(nil, rq, rq.Metadata, key.Bytes(), value)

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
	requesterID, err := l.checkRequestAndKeyValue(nil, rq, rq.Metadata, key.Bytes(), value)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}

func TestCheckRequestAndKeyValue_checkErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	value, _ := api.NewTestDocument(rng)
	key := cid.NewPseudoRandom(rng) // bad key, not hash of value
	rq := client.NewGetRequest(selfID, key)
	l := &Librarian{
		rt:  routing.NewEmpty(selfID, routing.NewDefaultParameters()),
		rqv: &alwaysRequestVerifier{},
		kc:  storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc: storage.NewHashKeyValueChecker(),
	}
	requesterID, err := l.checkRequestAndKeyValue(nil, rq, rq.Metadata, key.Bytes(), value)

	assert.Nil(t, requesterID)
	assert.NotNil(t, err)
}
