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


/**************
 * Benchmarks *
/*************/

var benchmarkCases = []struct {
	name           string
	plaintextSizes []int
}{
	{"small", []int{32, 64, 128, 256}},
	{"medium", []int{2048, 4096, 8192}},
	{"large", []int{256 * 1024, 512 * 1024, 2 * 1024 * 1024}},
}

func BenchmarkEncrypt(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkEncrypt(b, c.plaintextSizes) })
	}
}

func BenchmarkDecrypt(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkDecrypt(b, c.plaintextSizes) })
	}
}

func benchmarkEncrypt(b *testing.B, plaintextSizes []int) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)
	encrypter, err := NewEncrypter(keys)
	maybePanic(err)

	plaintexts := make([][]byte, len(plaintextSizes))
	totBytes := int64(0)
	for i, plaintextSize := range plaintextSizes {
		plaintexts[i] = make([]byte, plaintextSize)
		_, err := rng.Read(plaintexts[i])
		maybePanic(err)
		totBytes += int64(plaintextSize)
	}

	b.SetBytes(totBytes)
	for n := 0; n < b.N; n++ {
		for _, plaintext := range plaintexts {
			_, err = encrypter.Encrypt(plaintext, 0)
			maybePanic(err)
		}
	}
}

func benchmarkDecrypt(b *testing.B, plaintextSizes []int) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)

	decrypter, err := NewDecrypter(keys)
	maybePanic(err)

	encrypter, err := NewEncrypter(keys)
	maybePanic(err)

	ciphertexts := make([][]byte, len(plaintextSizes))
	totBytes := int64(0)
	for i, plaintextSize := range plaintextSizes {

		plaintext := make([]byte, plaintextSize)
		_, err := rng.Read(plaintext)
		maybePanic(err)

		ciphertexts[i], err = encrypter.Encrypt(plaintext, 0)
		totBytes += int64(len(ciphertexts[i]))
	}

	b.SetBytes(totBytes)
	for n := 0; n < b.N; n++ {
		for _, ciphertext := range ciphertexts {
			_, err = decrypter.Decrypt(ciphertext, 0)
			maybePanic(err)
		}
	}
}

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}
