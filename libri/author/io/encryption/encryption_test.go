package encryption

import (
	"testing"
	"io"
	"bytes"
	"math/rand"
	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := NewPseudoRandomKeys(rng)
	nPlaintextBytesPerPage, nPages := 32, 3

	ciphertext := new(bytes.Buffer)
	encrypter, err := NewEncrypter(noOpWriteCloser{ciphertext}, keys)
	assert.Nil(t, err)

	decrypter, err := NewDecrypter(noOpReadCloser{ciphertext}, keys)
	assert.Nil(t, err)

	for p := 0; p < nPages; p++ {

		encrypter.SetPageIndex(uint32(p))
		decrypter.SetPageIndex(uint32(p))

		plaintext1 := make([]byte, nPlaintextBytesPerPage)
		n0, err := rng.Read(plaintext1)
		assert.Equal(t, n0, nPlaintextBytesPerPage)
		assert.Nil(t, err)

		n1, err := encrypter.Write(plaintext1)
		assert.Nil(t, err)

		// ciphertext can be longer b/c of GCM auth overhead
		assert.True(t, n1 >= len(plaintext1))
		assert.True(t, ciphertext.Len() >= len(plaintext1))

		plaintext2 := new(bytes.Buffer)
		_, err = io.Copy(plaintext2, decrypter)
		assert.Nil(t, err)
		assert.Equal(t, plaintext1, plaintext2.Bytes())
	}
}

// noOpReadCloser implements a no-op Close method on top of an io.Reader.
type noOpReadCloser struct {
	io.Reader
}

func (noOpReadCloser) Close() error { return nil }

// noOpWriteCloser implements a no-op Close method on top of an io.Writer.
type noOpWriteCloser struct {
	io.Writer
}

func (noOpWriteCloser) Close() error { return nil }