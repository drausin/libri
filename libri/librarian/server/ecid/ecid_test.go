package ecid

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestECID_NewRandom(t *testing.T) {
	// since we're testing the other main functions in other tests, just make sure this runs
	// without error
	assert.NotNil(t, NewRandom())
}

func TestECID_NewPsuedoRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.True(t, val.Cmp(cid.LowerBound) >= 0)
		assert.True(t, val.Cmp(cid.UpperBound) <= 0)
	}
}

func TestECID_String(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.Equal(t, cid.Length*2, len(val.String())) // 32 hex-enc bytes = 64 chars
	}
}

func TestECID_Bytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.Equal(t, cid.Length, len(val.Bytes())) // should always be exactly 32 bytes
	}
}

func TestECID_Cmp(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.True(t, val.Cmp(cid.LowerBound) >= 0)
		assert.True(t, val.Cmp(cid.UpperBound) <= 0)
	}
}

func TestECID_Key_Sign(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	hash := sha256.Sum256([]byte("some value to sign"))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)

		// use Key to sign hash as (r, s)
		r, s, err := ecdsa.Sign(rng, val.Key(), hash[:])
		assert.Nil(t, err)

		// verify hash (r, s) signature
		assert.True(t, ecdsa.Verify(&val.Key().PublicKey, hash[:], r, s))
	}
}
