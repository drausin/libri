package encryption

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"math/rand"
	"github.com/stretchr/testify/assert"
)

func TestNewEncryptedMetadata_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	ciphertext, ciphertextMAC := api.RandBytes(rng, 64), api.RandBytes(rng, 32)
	m, err := NewEncryptedMetadata(ciphertext, ciphertextMAC)
	assert.Nil(t, err)
	assert.Equal(t, ciphertext, m.Ciphertext)
	assert.Equal(t, ciphertextMAC, m.CiphertextMAC)
}

func TestNewEncryptedMetadata_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	// check errors on invalid ciphertext
	ciphertext1, ciphertextMAC1 := []byte{}, api.RandBytes(rng, 32)
	m1, err := NewEncryptedMetadata(ciphertext1, ciphertextMAC1)
	assert.NotNil(t, err)
	assert.Nil(t, m1)

	// check errors on invalid ciphertextMAC
	ciphertext2, ciphertextMAC2 := api.RandBytes(rng, 64), api.RandBytes(rng, 8)
	m2, err := NewEncryptedMetadata(ciphertext2, ciphertextMAC2)
	assert.NotNil(t, err)
	assert.Nil(t, m2)
}

func TestEncryptDecryptMetadata(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomKeys(rng)
	mediaType := "application/x-pdf"
	m1, err := api.NewEntryMetadata(
		mediaType,
		1,
		api.RandBytes(rng, 32),
		2,
		api.RandBytes(rng, 32),
	)
	assert.Nil(t, err)

	em, err := EncryptMetadata(m1, keys)
	assert.Nil(t, err)

	m2, err := DecryptMetadata(em, keys)
	assert.Nil(t, err)

	assert.Equal(t, m1, m2)
}

func TestEncryptMetadata_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys1 := NewPseudoRandomKeys(rng)

	// check bad api.Metadata triggers error
	em1, err := EncryptMetadata(nil, keys1)
	assert.NotNil(t, err)
	assert.Nil(t, em1)

	mediaType := "application/x-pdf"
	m, err := api.NewEntryMetadata(
		mediaType,
		1,
		api.RandBytes(rng, 32),
		2,
		api.RandBytes(rng, 32),
	)
	assert.Nil(t, err)

	// check missing AES key triggers error
	em2, err := EncryptMetadata(m, &Keys{})
	assert.NotNil(t, err)
	assert.Nil(t, em2)
}

func TestDecryptMetadata_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	em1, err := NewEncryptedMetadata(api.RandBytes(rng, 64), api.RandBytes(rng, 32))
	assert.Nil(t, err)

	// check bad HMAC key triggers error
	m1, err := DecryptMetadata(em1, &Keys{})
	assert.NotNil(t, err)
	assert.Nil(t, m1)

	em2, err := NewEncryptedMetadata(api.RandBytes(rng, 64), api.RandBytes(rng, 32))
	assert.Nil(t, err)
	keys2 := NewPseudoRandomKeys(rng)

	// check different MAC triggers error;
	m2, err := DecryptMetadata(em2, keys2)
	assert.Equal(t, UnexpectedMACErr, err)
	assert.Nil(t, m2)

	keys3 := NewPseudoRandomKeys(rng)  // valid, but not connected to ciphertext
	ciphertext3 := api.RandBytes(rng, 64)
	ciphertextMAC3, err := HMAC(ciphertext3, keys3.HMACKey)
	assert.Nil(t, err)
	em3, err := NewEncryptedMetadata(ciphertext3, ciphertextMAC3)
	assert.Nil(t, err)

	// check bad HMAC key triggers error
	m3, err := DecryptMetadata(em3, keys3)
	assert.NotNil(t, err)
	assert.Nil(t, m3)

	keys4 := NewPseudoRandomKeys(rng)  // valid, but not connected to ciphertext
	ciphertext4 := api.RandBytes(rng, 64)
	ciphertextMAC4, err := HMAC(ciphertext3, keys3.HMACKey)
	assert.Nil(t, err)
	em4, err := NewEncryptedMetadata(ciphertext4, ciphertextMAC4)
	assert.Nil(t, err)

	// check bad MetadataIV triggers error
	m4, err := DecryptMetadata(em4, keys4)
	assert.NotNil(t, err)
	assert.Nil(t, m4)
}