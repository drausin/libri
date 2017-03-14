package client

import (
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/stretchr/testify/assert"
)

func TestNewRequestMetadata(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	m := NewRequestMetadata(peerID)
	pub, err := ecid.FromPublicKeyBytes(m.PubKey)
	assert.Nil(t, err)

	assert.Equal(t, cid.Length, len(m.RequestId))
	assert.NotNil(t, m.PubKey)
	assert.Equal(t, &peerID.Key().PublicKey, pub)
}

func TestNewIntroduceRequest(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID, nPeers := ecid.NewPseudoRandom(rng), uint(8)

	rq := NewIntroduceRequest(selfID, &api.PeerAddress{}, nPeers)
	assert.NotNil(t, rq.Metadata)
	assert.NotNil(t, rq.Self)
	assert.Equal(t, uint32(nPeers), rq.NumPeers)
}

func TestNewFindRequest(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key, nPeers := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), uint(8)
	rq := NewFindRequest(peerID, key, nPeers)
	assert.NotNil(t, rq.Metadata)
	assert.Equal(t, key.Bytes(), rq.Key)
	assert.Equal(t, uint32(nPeers), rq.NumPeers)
}

func TestNewStoreRequest(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)
	rq := NewStoreRequest(peerID, key, value)
	assert.NotNil(t, rq.Metadata)
	assert.Equal(t, key.Bytes(), rq.Key)
	assert.Equal(t, value, rq.Value)
}

func TestNewGetRequest(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	rq := NewGetRequest(peerID, key)
	assert.NotNil(t, rq.Metadata)
	assert.Equal(t, key.Bytes(), rq.Key)
}

func TestNewPutRequest(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)
	rq := NewPutRequest(peerID, key, value)
	assert.NotNil(t, rq.Metadata)
	assert.Equal(t, key.Bytes(), rq.Key)
	assert.Equal(t, value, rq.Value)
}
