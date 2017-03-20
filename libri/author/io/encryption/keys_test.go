package encryption

import (
	"testing"
	"math/rand"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	k1 := NewRandomKeys()

	assert.NotNil(t, k1.AESKey)
	assert.NotNil(t, k1.PageIVSeed)
	assert.NotNil(t, k1.PageHMACKey)
	assert.NotNil(t, k1.MetadataIV)

	// check that first 8 bytes of adjacent fields are different
	assert.NotEqual(t, k1.AESKey[:8], k1.PageIVSeed[:8])
	assert.NotEqual(t, k1.PageIVSeed[:8], k1.PageHMACKey[:8])
	assert.NotEqual(t, k1.PageHMACKey[:8], k1.MetadataIV[:8])
}

func TestMarshallUnmarshall_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	k1 := NewPseudoRandomKeys(rng)
	k2, err := Unmarshal(Marshal(k1))
	assert.Nil(t, err)
	assert.Equal(t, k1, k2)
}
