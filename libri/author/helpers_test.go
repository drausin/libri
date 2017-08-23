package author

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/rand"
	"testing"

	"errors"

	"net"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/stretchr/testify/assert"
)

func TestEnvelopeKeySampler_Sample_ok(t *testing.T) {
	authorKeys, selfReaderKeys := keychain.New(1), keychain.New(1)
	s := &envelopeKeySamplerImpl{
		authorKeys:     authorKeys,
		selfReaderKeys: selfReaderKeys,
	}
	authPubBytes, srPubBytes, kek1, eek, err := s.sample()
	assert.Nil(t, err)
	assert.NotNil(t, authPubBytes)
	assert.NotNil(t, srPubBytes)
	assert.NotNil(t, kek1)
	assert.NotNil(t, eek)
	authPub, err := ecid.FromPublicKeyBytes(authPubBytes)
	assert.Nil(t, err)

	// construct keys other way from selfReader private key and author public key
	srPriv, in := selfReaderKeys.Get(srPubBytes)
	assert.True(t, in)
	assert.NotNil(t, srPriv)
	kek2, err := enc.NewKEK(srPriv.Key(), authPub)
	assert.Nil(t, err)

	// check keys constructed each way are equal
	assert.Equal(t, kek1, kek2)
}

func TestEnvelopeKeySampler_Sample_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	// authorKeys.Sample() error should bubble up
	s1 := &envelopeKeySamplerImpl{
		authorKeys:     &fixedKeychain{sampleErr: errors.New("some Sample error")},
		selfReaderKeys: keychain.New(3),
	}
	aPB, srPB, kek, eek, err := s1.sample()
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, kek)
	assert.Nil(t, eek)

	// selfReaderKeys.Sample() error should bubble up
	s2 := &envelopeKeySamplerImpl{
		authorKeys:     keychain.New(3),
		selfReaderKeys: &fixedKeychain{sampleErr: errors.New("some Sample error")},
	}
	aPB, srPB, kek, eek, err = s2.sample()
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, kek)
	assert.Nil(t, eek)

	// NewKEK() error should bubble up
	offCurvePriv, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	s3 := &envelopeKeySamplerImpl{
		authorKeys: &fixedKeychain{ // will cause error in NewKEK
			sampleID: ecid.FromPrivateKey(offCurvePriv),
		},
		selfReaderKeys: keychain.New(3),
	}
	aPB, srPB, kek, eek, err = s3.sample()
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, kek)
	assert.Nil(t, eek)

	// too hard/annoying to create error in NewEEK()
}

func TestGetLibrarianHealthClients(t *testing.T) {
	librarianAddrs := []*net.TCPAddr{
		{IP: net.ParseIP("127.0.0.1"), Port: 20100},
		{IP: net.ParseIP("127.0.0.1"), Port: 20101},
	}
	healthClients, err := getLibrarianHealthClients(librarianAddrs)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(healthClients))
	_, in := healthClients["127.0.0.1:20100"]
	assert.True(t, in)
	_, in = healthClients["127.0.0.1:20101"]
	assert.True(t, in)
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
