package enc

import (
	"math/rand"
	"testing"

	"crypto/ecdsa"
	"crypto/elliptic"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/stretchr/testify/assert"
)

func TestNewKeys_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	authorPriv := ecid.NewPseudoRandom(rng)
	readerPriv := ecid.NewPseudoRandom(rng)

	k1, err := NewKeys(authorPriv.Key(), &readerPriv.Key().PublicKey)
	assert.Nil(t, err)

	assert.NotNil(t, k1.AESKey)
	assert.NotNil(t, k1.PageIVSeed)
	assert.NotNil(t, k1.HMACKey)
	assert.NotNil(t, k1.MetadataIV)

	// check that first 8 bytes of adjacent fields are different
	assert.NotEqual(t, k1.AESKey[:8], k1.PageIVSeed[:8])
	assert.NotEqual(t, k1.PageIVSeed[:8], k1.HMACKey[:8])
	assert.NotEqual(t, k1.HMACKey[:8], k1.MetadataIV[:8])

	k2, err := NewKeys(readerPriv.Key(), &authorPriv.Key().PublicKey)
	assert.Nil(t, err)

	// check that ECDH shared secred + HKDF create same keys
	assert.Equal(t, k1, k2)
}

func TestNewKeys_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	privOffCurve, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	privOnCurve := ecid.NewPseudoRandom(rng)

	// check that off-curve private key results in error
	k1, err := NewKeys(privOffCurve, &privOnCurve.Key().PublicKey)
	assert.NotNil(t, err)
	assert.Nil(t, k1)

	// check that off-surve public key results in error
	k2, err := NewKeys(privOnCurve.Key(), &privOffCurve.PublicKey)
	assert.NotNil(t, err)
	assert.Nil(t, k2)
}

func TestMarshallUnmarshall_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	k1, _, _ := NewPseudoRandomKeys(rng)
	k2, err := Unmarshal(Marshal(k1))
	assert.Nil(t, err)
	assert.Equal(t, k1, k2)
}
