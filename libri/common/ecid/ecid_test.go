package ecid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	cerrors "github.com/drausin/libri/libri/common/errors"
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
		assert.True(t, val.ID().Cmp(id.LowerBound) >= 0)
		assert.True(t, val.ID().Cmp(id.UpperBound) <= 0)
	}
}

func TestEcid_String(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.Equal(t, id.Length*2, len(val.String())) // 32 hex-enc bytes = 64 chars
	}
}

func TestEcid_Bytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.Equal(t, id.Length, len(val.Bytes())) // should always be exactly 32 bytes
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
	assert.Equal(t, i.(*ecid).id.Distance(j.ID()), i.Distance(j))
	assert.Equal(t, i.(*ecid).id.Distance(k.ID()), i.Distance(k))
}

func TestEcid_Cmp(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 10; c++ {
		val := NewPseudoRandom(rng)
		assert.True(t, val.ID().Cmp(id.LowerBound) >= 0)
		assert.True(t, val.ID().Cmp(id.UpperBound) <= 0)
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

func TestEcid_SaveLoad(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	i1 := NewPseudoRandom(rng)
	tmpDerFilepath, err := ioutil.TempFile("", "test-priv-key")
	defer func() {
		err := os.Remove(tmpDerFilepath.Name())
		cerrors.MaybePanic(err)
	}()
	assert.Nil(t, err)
	err = i1.Save(tmpDerFilepath.Name())
	assert.Nil(t, err)

	i2, err := FromPrivateKeyFile(tmpDerFilepath.Name())
	assert.Nil(t, err)
	assert.Equal(t, i1, i2)
}

func TestFromPrivateKey_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badPriv, err := ecdsa.GenerateKey(elliptic.P256(), rng) // wrong curve
	assert.Nil(t, err)
	i, err := FromPrivateKey(badPriv)
	assert.Equal(t, ErrPrivKeyUnexpectedCurve, err)
	assert.Nil(t, i)
}

func TestFromPublicKeyBytes_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	okPub := NewPseudoRandom(rng).Key().PublicKey
	badPriv, err := ecdsa.GenerateKey(elliptic.P256(), rng) // wrong curve
	assert.Nil(t, err)
	cases := [][]byte{
		elliptic.Marshal(Curve, okPub.X, okPub.Y),     // uncompressed representation
		make([]byte, 32),                              // wrong length
		make([]byte, 33),                              // very unlikely that first byte is required 2 or 3
		elliptic.Marshal(Curve, badPriv.X, badPriv.Y), // wrong curve
	}

	for i, c := range cases {
		info := fmt.Sprintf("case: %d", i)
		pub, err := FromPublicKeyBytes(c)
		assert.Nil(t, pub, info)
		assert.NotNil(t, err, info)
	}
}

func TestToFromPublicKeyBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for c := 0; c < 64; c++ {
		i := NewPseudoRandom(rng)
		pubBytes := i.PublicKeyBytes()
		assert.Len(t, pubBytes, 33)
		pub, err := FromPublicKeyBytes(pubBytes)
		assert.Nil(t, err)
		assert.Equal(t, i.Key().X, pub.X)
		assert.Equal(t, i.Key().Y, pub.Y)
	}
}

func TestMarshalUnmarshallCompressed_shortX(t *testing.T) {
	// construct a valid point where the X value has fewer than 32 bytes
	xBytes, err := hex.DecodeString(
		"ce523fd0fac313b412446e39d3eb139af32a29c66ca60af41f4e64a7227e18")
	assert.Nil(t, err)
	x := new(big.Int)
	x.SetBytes(xBytes)

	yBytes, err := hex.DecodeString(
		"ec3c7e9d1822338d008c53e7996309cfd5fd51d088911ebbb8a2f129a7fb40d9")
	assert.Nil(t, err)
	y := new(big.Int)
	y.SetBytes(yBytes)

	pub1 := &ecdsa.PublicKey{Curve: Curve, X: x, Y: y}

	compressed := marshalCompressed(pub1)
	pub2, err := unmarshalCompressed(compressed)
	assert.Nil(t, err)
	assert.Equal(t, pub1, pub2)
}
