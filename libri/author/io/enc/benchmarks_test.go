package enc

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/errors"
)

const (
	KB = 1024
	MB = 1024 * KB
)

var (
	smallPlaintextSizes  = []int{128}
	mediumPlaintextSizes = []int{KB}
	largePlaintextSizes  = []int{MB}
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
	b.StopTimer()
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)
	encrypter, err := NewEncrypter(keys)
	errors.MaybePanic(err)

	plaintexts := make([][]byte, len(plaintextSizes))
	totBytes := int64(0)
	for i, plaintextSize := range plaintextSizes {
		plaintexts[i] = make([]byte, plaintextSize)
		_, err = rng.Read(plaintexts[i])
		errors.MaybePanic(err)
		totBytes += int64(plaintextSize)
	}

	b.SetBytes(totBytes)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for _, plaintext := range plaintexts {
			_, err = encrypter.Encrypt(plaintext, 0)
			errors.MaybePanic(err)
		}
	}
}

func benchmarkDecrypt(b *testing.B, plaintextSizes []int) {
	b.StopTimer()
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomEEK(rng)

	decrypter, err := NewDecrypter(keys)
	errors.MaybePanic(err)

	encrypter, err := NewEncrypter(keys)
	errors.MaybePanic(err)

	ciphertexts := make([][]byte, len(plaintextSizes))
	totBytes := int64(0)
	for i, plaintextSize := range plaintextSizes {

		plaintext := make([]byte, plaintextSize)
		_, err = rng.Read(plaintext)
		errors.MaybePanic(err)

		ciphertexts[i], err = encrypter.Encrypt(plaintext, 0)
		errors.MaybePanic(err)
		totBytes += int64(len(ciphertexts[i]))
	}

	b.SetBytes(totBytes)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for _, ciphertext := range ciphertexts {
			_, err = decrypter.Decrypt(ciphertext, 0)
			errors.MaybePanic(err)
		}
	}
}
