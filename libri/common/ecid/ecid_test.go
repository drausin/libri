package ecid

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"math/rand"
	"testing"

	"crypto/elliptic"

	"errors"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/stretchr/testify/assert"
)

func TestEcid_NewRandom_ok(t *testing.T) {
	// since we're testing the other main functions in other tests, just make sure this runs
	// without error
	c1, c2 := NewRandom(), NewRandom()
	assert.NotNil(t, c1)
	assert.NotEqual(t, c1, c2)
	assert.NotEqual(t, c1.ID(), c2.ID())
}

type truncReader struct{}

func (tr *truncReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("unable to read anything")
}

func TestEcid_NewRandom_err(t *testing.T) {
	assert.Panics(t, func() {
		newRandom(&truncReader{})
	})
}

func TestEcid_NewPsuedoRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.True(t, val.Cmp(cid.LowerBound) >= 0)
		assert.True(t, val.Cmp(cid.UpperBound) <= 0)
	}
}

func TestEcid_String(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.Equal(t, cid.Length*2, len(val.String())) // 32 hex-enc bytes = 64 chars
	}
}

func TestEcid_Bytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.Equal(t, cid.Length, len(val.Bytes())) // should always be exactly 32 bytes
	}
}

func TestEcid_Int(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	i := NewPseudoRandom(rng)
	assert.Equal(t, i.(*ecid).id.Int(), i.Int())
}

func TestEcid_Distance(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	i, j, k := NewPseudoRandom(rng), NewPseudoRandom(rng), NewPseudoRandom(rng)
	assert.Equal(t, i.(*ecid).id.Distance(j), i.Distance(j))
	assert.Equal(t, i.(*ecid).id.Distance(k), i.Distance(k))
}

func TestEcid_Cmp(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.True(t, val.Cmp(cid.LowerBound) >= 0)
		assert.True(t, val.Cmp(cid.UpperBound) <= 0)
	}
}

func TestEcid_Key_Sign(t *testing.T) {
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

func TestEcid_ID(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	i := NewPseudoRandom(rng)
	assert.NotNil(t, i.ID())
	assert.Equal(t, i.(*ecid).id, i.ID())
}

func TestFromPublicKeyBytes_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	priv, err := ecdsa.GenerateKey(Curve, rng)
	assert.Nil(t, err)
	pubBytes := elliptic.Marshal(Curve, priv.X, priv.Y)

	pub, err := FromPublicKeyBytes(pubBytes)
	assert.Nil(t, err)
	assert.Equal(t, Curve, pub.Curve)
	assert.Equal(t, priv.X, pub.X)
	assert.Equal(t, priv.Y, pub.Y)
}

func TestFromPublicKeyBytes_err(t *testing.T) {
	short := make([]byte, 1)
	pub, err := FromPublicKeyBytes(short)
	assert.Nil(t, pub)
	assert.NotNil(t, err)
}

func TestToPublicKeyBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	i := NewPseudoRandom(rng)
	pub, err := FromPublicKeyBytes(ToPublicKeyBytes(i))
	assert.Nil(t, err)
	assert.Equal(t, i.Key().X, pub.X)
	assert.Equal(t, i.Key().Y, pub.Y)
}
