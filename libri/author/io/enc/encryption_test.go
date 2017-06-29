package enc

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEncrypter_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)
	enc, err := NewEncrypter(keys)
	assert.Nil(t, err)
	assert.NotNil(t, enc.(*encrypter).gcmCipher)
	assert.NotNil(t, enc.(*encrypter).pageIVMAC)
}

func TestNewEncrypter_err(t *testing.T) {
	enc, err := NewEncrypter(&EEK{})
	assert.NotNil(t, err)
	assert.Nil(t, enc)
}

func TestNewDecrypter_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)
	enc, err := NewDecrypter(keys)
	assert.Nil(t, err)
	assert.NotNil(t, enc.(*decrypter).gcmCipher)
	assert.NotNil(t, enc.(*decrypter).pageIVMAC)
}

func TestNewDecrypter_err(t *testing.T) {
	enc, err := NewDecrypter(&EEK{})
	assert.NotNil(t, err)
	assert.Nil(t, enc)
}

func TestEncryptDecrypt(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)
	nPlaintextBytesPerPage, nPages := 32, uint32(3)

	encrypter, err := NewEncrypter(keys)
	assert.Nil(t, err)

	decrypter, err := NewDecrypter(keys)
	assert.Nil(t, err)

	for p := uint32(0); p < nPages; p++ {

		plaintext1 := make([]byte, nPlaintextBytesPerPage)
		n0, err := rng.Read(plaintext1)
		assert.Equal(t, n0, nPlaintextBytesPerPage)
		assert.Nil(t, err)

		ciphertext, err := encrypter.Encrypt(plaintext1, p)
		assert.Nil(t, err)

		// ciphertext can be longer b/c of GCM auth overhead
		assert.True(t, len(ciphertext) >= len(plaintext1))

		// check that page number matters
		diffCiphertext, err := encrypter.Encrypt(plaintext1, p+1)
		assert.Nil(t, err)
		assert.NotEqual(t, ciphertext, diffCiphertext)

		// check that decrypted plaintext matches original
		plaintext2, err := decrypter.Decrypt(ciphertext, p)
		assert.Nil(t, err)
		assert.Equal(t, plaintext1, plaintext2)
	}
}
