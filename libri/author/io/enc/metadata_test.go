package enc

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
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

func TestMetadataEncDec_EncryptDecrypt(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := NewPseudoRandomKeys(rng)
	mediaType := "application/x-pdf"
	m1, err := api.NewEntryMetadata(
		mediaType,
		1,
		api.RandBytes(rng, 32),
		2,
		api.RandBytes(rng, 32),
	)
	assert.Nil(t, err)

	me := metadataEncDec{}
	em, err := me.Encrypt(m1, keys)
	assert.Nil(t, err)

	md := metadataEncDec{}
	m2, err := md.Decrypt(em, keys)
	assert.Nil(t, err)

	assert.Equal(t, m1, m2)
}

func TestMetadataEncDec_Encrypt_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys1, _, _ := NewPseudoRandomKeys(rng)
	med := NewMetadataEncrypterDecrypter()

	// check bad api.Metadata triggers error
	em1, err := med.Encrypt(nil, keys1)
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
	keys2, _, _ := NewPseudoRandomKeys(rng)
	keys2.AESKey = nil
	em2, err := med.Encrypt(m, keys2)
	assert.NotNil(t, err)
	assert.Nil(t, em2)
}

func TestMetadataEncDec_Decrypt_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	med := NewMetadataEncrypterDecrypter()

	em1, err := NewEncryptedMetadata(api.RandBytes(rng, 64), api.RandBytes(rng, 32))
	assert.Nil(t, err)
	keys1, _, _ := NewPseudoRandomKeys(rng)
	keys1.HMACKey = nil

	// check bad HMAC key triggers error
	m1, err := med.Decrypt(em1, keys1)
	assert.NotNil(t, err)
	assert.Nil(t, m1)

	em2, err := NewEncryptedMetadata(api.RandBytes(rng, 64), api.RandBytes(rng, 32))
	assert.Nil(t, err)
	keys2, _, _ := NewPseudoRandomKeys(rng)  // valid, but not connected to ciphertext

	// check different MAC triggers error;
	m2, err := med.Decrypt(em2, keys2)
	assert.Equal(t, ErrUnexpectedMAC, err)
	assert.Nil(t, m2)

	keys3, _, _ := NewPseudoRandomKeys(rng)
	keys3.AESKey = nil
	ciphertext3 := api.RandBytes(rng, 64)
	ciphertextMAC3 := HMAC(ciphertext3, keys3.HMACKey)
	em3, err := NewEncryptedMetadata(ciphertext3, ciphertextMAC3)
	assert.Nil(t, err)

	// check bad AES key triggers error
	m3, err := med.Decrypt(em3, keys3)
	assert.NotNil(t, err)
	assert.Nil(t, m3)

	keys4, _, _ := NewPseudoRandomKeys(rng) // valid, but not connected to ciphertext
	ciphertext4 := api.RandBytes(rng, 64)
	ciphertextMAC4 := HMAC(ciphertext4, keys3.HMACKey)
	em4, err := NewEncryptedMetadata(ciphertext4, ciphertextMAC4)
	assert.Nil(t, err)

	// check bad MetadataIV triggers error
	m4, err := med.Decrypt(em4, keys4)
	assert.NotNil(t, err)
	assert.Nil(t, m4)
}
