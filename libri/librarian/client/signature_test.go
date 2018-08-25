package client

import (
	"math/rand"
	"testing"

	"fmt"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestSignatureClaims_Valid_ok(t *testing.T) {
	// all of these should be considered valid hashes
	cases := []*Claims{
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
	cases := []*Claims{
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
	orgID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)

	signer, verifier := NewECDSASigner(peerID.Key()), NewVerifier()

	cases := []proto.Message{
		NewFindRequest(peerID, orgID, key, 20),
		NewStoreRequest(peerID, orgID, key, value),
		NewGetRequest(peerID, orgID, key),
		NewPutRequest(peerID, orgID, key, value),
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

	signer := NewECDSASigner(peerID.Key())
	_, err := signer.Sign(nil)
	assert.NotNil(t, err) // protobuf needs to be not-nil
}

func TestEcdsaVerifer_Verify_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	key, value := id.NewPseudoRandom(rng), make([]byte, 512)
	rng.Read(value)

	signer, verifier := NewECDSASigner(peerID.Key()), NewVerifier()
	message := NewFindRequest(peerID, orgID, key, 20)
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
		{encToken, peerID, NewFindRequest(peerID, orgID, key, 10)}, // NPeers
		{encToken, peerID, NewFindRequest(peerID, orgID, key, 20)}, // Metadata.RequestID

		// different peer
		{encToken, ecid.NewPseudoRandom(rng), message},
	}
	for _, c := range errCases {
		assert.NotNil(t, verifier.Verify(c.encToken, &c.peerID.Key().PublicKey, c.m))
	}

	assert.Panics(t, func() {
		err := verifier.Verify(encToken, nil, message) // can't have nil key
		errors.MaybePanic(err)
	})
}

func TestTestNoOpSigner_Sign(t *testing.T) {
	s := &TestNoOpSigner{}
	token, err := s.Sign(nil)
	assert.NotNil(t, token)
	assert.Nil(t, err)
}

func TestTestErrSigner_Sign(t *testing.T) {
	s := &TestErrSigner{}
	token, err := s.Sign(nil)
	assert.Equal(t, "", token)
	assert.NotNil(t, err)
}

// TestEntryBytes is mostly used for confirming that certain messages are serialized to the same
// binary representation in this Go implementation as in the Javascript one.
func TestEntryBytes(t *testing.T) {
	dummy := []byte{0, 1, 2}
	page := &api.Page{
		AuthorPublicKey: dummy,
		Index:           0,
		Ciphertext:      dummy,
		CiphertextMac:   dummy,
	}
	pageBytes, err := proto.Marshal(page)
	assert.Nil(t, err)
	assert.Equal(t, "0a030001021a030001022203000102", fmt.Sprintf("%x", pageBytes))

	entry1 := &api.Entry{
		AuthorPublicKey:       dummy,
		CreatedTime:           1,
		MetadataCiphertext:    dummy,
		MetadataCiphertextMac: dummy,
	}
	entry1Bytes, err := proto.Marshal(entry1)
	assert.Nil(t, err)
	assert.Equal(t, "0a0300010220012a030001023203000102", fmt.Sprintf("%x", entry1Bytes))

	entry2 := &api.Entry{
		CreatedTime: 1,
		Page:        page,
	}
	entry2Bytes, err := proto.Marshal(entry2)
	assert.Nil(t, err)
	assert.Equal(t, "120f0a030001021a0300010222030001022001", fmt.Sprintf("%x", entry2Bytes))
}
