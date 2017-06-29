package enc

import (
	"testing"
	"math/rand"
)

const (
	KB = 1024
	MB = 1024 * KB
)

var (
	smallPlaintextSizes  = []int{32, 64, 128, 256}
	mediumPlaintextSizes = []int{2048, 4096, 8192}
	largePlaintextSizes  = []int{256 * KB, 512 * KB, MB}
)

var benchmarkCases = []struct {
	name           string
	plaintextSizes []int
}{
	{"small", smallPlaintextSizes},
	{"medium", mediumPlaintextSizes},
	{"large", largePlaintextSizes},
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
