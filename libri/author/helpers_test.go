package author

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"errors."
	"github.com/stretchr/testify/assert"
)

func TestEnvelopeKeySampler_Sample_ok(t *testing.T) {
	authorKeys, selfReaderKeys := keychain.New(3), keychain.New(3)
	s := &envelopeKeySamplerImpl{
		authorKeys:     authorKeys,
		selfReaderKeys: selfReaderKeys,
	}
	authPubBytes, srPubBytes, encKeys1, err := s.sample()
	assert.Nil(t, err)
	assert.NotNil(t, authPubBytes)
	assert.NotNil(t, srPubBytes)
	assert.NotNil(t, encKeys1)
	authPub, err := ecid.FromPublicKeyBytes(authPubBytes)
	assert.Nil(t, err)

	// construct keys other way from selfReader private key and author public key
	srPriv, in := selfReaderKeys.Get(srPubBytes)
	assert.True(t, in)
	assert.NotNil(t, srPriv)
	encKeys2, err := enc.NewKeys(srPriv.Key(), authPub)
	assert.Nil(t, err)

	// check keys constructed each way are equal
	assert.Equal(t, encKeys1, encKeys2)
}

func TestEnvelopeKeySampler_Sample_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	s1 := &envelopeKeySamplerImpl{
		authorKeys:     &fixedKeychain{sampleErr: errors.New("some Sample error")},
		selfReaderKeys: keychain.New(3),
	}
	aPB, srPB, eK, err := s1.sample()
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, eK)

	s2 := &envelopeKeySamplerImpl{
		authorKeys:     keychain.New(3),
		selfReaderKeys: &fixedKeychain{sampleErr: errors.New("some Sample error")},
	}
	aPB, srPB, eK, err = s2.sample()
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, eK)

	offCurvePriv, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	s3 := &envelopeKeySamplerImpl{
		authorKeys:     &fixedKeychain{sampleID: ecid.FromPrivateKey(offCurvePriv)},
		selfReaderKeys: keychain.New(3),
	}
	aPB, srPB, eK, err = s3.sample()
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, eK)
}

type fixedKeychain struct {
	sampleID  ecid.ID
	sampleErr error
}

func (f *fixedKeychain) Sample() (ecid.ID, error) {
	return f.sampleID, f.sampleErr
}

func (f *fixedKeychain) Get(publicKey []byte) (ecid.ID, bool) {
	return nil, false
}

func (f *fixedKeychain) Len() int {
	return 0
}
