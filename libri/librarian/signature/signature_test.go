package signature

import (
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestSignatureClaims_Valid_ok(t *testing.T) {
	// all of these should be considered valid hashes
	cases := []*SignatureClaims{
		{"n4bQgYhMfWWaL-qgxVrQFaO_TxsrC4Is0V1sFbDwCgg="},
		{"9nITsSKl1ELSuTvajMRcVkpw7F0qTg6Vu1hc8ZmGnJg="},
		{"-MAqRWZ-E5DpcCh23U3GwAZuSbXNqm7ByD59iL6S4uI="},
		{"47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU="},
	}
	for _, c := range cases {
		assert.Nil(t, c.Valid())
	}
}

func TestSignatureClaims_Valid_err(t *testing.T) {
	// none of these is valid
	cases := []*SignatureClaims{
		{"n4bQgYhMfWWaL-qgxVrQFaO_TxsrC4Is0V1sFbDwCgga"},       // missing last =
		{"n4bQgYhMfWWaL+qgxVrQFaO_TxsrC4Is0V1sFbDwCgga"},       // + part of non-url base-64
		{"9nITsSKl1ELSuTvajMRcVkpw7F0qTg6Vu1hc8ZmGnJg"},        // too short
		{"9nITsSKl1ELSuTvajMRcVkpw7F0qTg6Vu1hc8ZmGnJgggggggg"}, // too long
		{""},            // too short
		{"test *&*&*&"}, // invalid chars
	}
	for _, c := range cases {
		assert.NotNil(t, c.Valid())
	}
}

func TestEcdsaSignerVerifer_SignVerify_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	key, value := cid.NewPseudoRandom(rng), make([]byte, 512)
	rng.Read(value)

	signer, verifier := NewSigner(peerID.Key()), NewVerifier()

	cases := []proto.Message{
		api.NewFindRequest(peerID, key, 20),
		api.NewStoreRequest(peerID, key, value),
		api.NewGetRequest(peerID, key),
		api.NewPutRequest(peerID, key, value),
	}
	for _, c := range cases {
		encToken, err := signer.Sign(c)
		assert.Nil(t, err)
		err = verifier.Verify(encToken, &peerID.Key().PublicKey, c)
		assert.Nil(t, err)
	}
}

func TestEcdsaSigner_Sign_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)

	signer := NewSigner(peerID.Key())
	_, err := signer.Sign(nil)
	assert.NotNil(t, err) // protobuf needs to be not-nil
}

func TestEcdsaVerifer_Verify_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	key, value := cid.NewPseudoRandom(rng), make([]byte, 512)
	rng.Read(value)

	signer, verifier := NewSigner(peerID.Key()), NewVerifier()
	message := api.NewFindRequest(peerID, key, 20)
	encToken, err := signer.Sign(message)
	assert.Nil(t, err)

	// none of these should verify
	errCases := []struct {
		encToken string
		peerID   ecid.ID
		m        proto.Message
	}{
		// zero values not allowed
		{encToken, peerID, nil},
		{"", peerID, message},

		// different messages
		{encToken, peerID, api.NewFindRequest(peerID, key, 10)}, // NPeers
		{encToken, peerID, api.NewFindRequest(peerID, key, 20)}, // Metadata.RequestID

		// different peer
		{encToken, ecid.NewPseudoRandom(rng), message},
	}
	for _, c := range errCases {
		assert.NotNil(t, verifier.Verify(c.encToken, &c.peerID.Key().PublicKey, c.m))
	}

	assert.Panics(t, func() {
		verifier.Verify(encToken, nil, message)
	})
}
