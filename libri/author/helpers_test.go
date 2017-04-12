package author

import (
	"testing"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/pkg/errors"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/rand"
)

func TestSampleSelfReaderKeys_ok(t *testing.T) {
	authorKeys, selfReaderKeys := keychain.New(3), keychain.New(3)
	authPubBytes, srPubBytes, encKeys1, err := sampleSelfReaderKeys(authorKeys, selfReaderKeys)
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


func TestSampleSelfReaderKeys_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	authorKeys1 := &fixedKeychain{sampleErr: errors.New("some Sample error")}
	selfReaderKeys1 := keychain.New(3)
	aPB, srPB, eK, err := sampleSelfReaderKeys(authorKeys1, selfReaderKeys1)
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, eK)

	authorKeys2 := keychain.New(3)
	selfReaderKeys2 := &fixedKeychain{sampleErr: errors.New("some Sample error")}
	aPB, srPB, eK, err = sampleSelfReaderKeys(authorKeys2, selfReaderKeys2)
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, eK)

	offCurvePriv, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	authorKeys3 := &fixedKeychain{sampleID: ecid.FromPrivateKey(offCurvePriv)}
	selfReaderKeys3 := keychain.New(3)
	aPB, srPB, eK, err = sampleSelfReaderKeys(authorKeys3, selfReaderKeys3)
	assert.NotNil(t, err)
	assert.Nil(t, aPB)
	assert.Nil(t, srPB)
	assert.Nil(t, eK)
}

type fixedKeychain struct {
	sampleID ecid.ID
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