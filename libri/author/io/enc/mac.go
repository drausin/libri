package enc

import (
	"hash"
	"crypto/hmac"
	"crypto/sha256"
	"io"
)

// MAC wraps a hash function to return a message authentication code (MAC) and the total number
// of bytes it has digested.
type MAC interface {
	io.Writer

	// Sum returns the MAC after writing the given bytes.
	Sum(in []byte) []byte

	// Reset resets the MAC.
	Reset()

	// MessageSize returns the total number of digested bytes.
	MessageSize() uint64
}

type sizeHMAC struct {
	inner hash.Hash
	size  uint64
}

// NewHMAC returns a MAC internally using an HMAC-256 with a a given key.
func NewHMAC(hmacKey []byte) MAC {
	return &sizeHMAC{
		inner: hmac.New(sha256.New, hmacKey),
	}
}

func (h *sizeHMAC) Write(p []byte) (int, error) {
	n, err := h.inner.Write(p)
	h.size += uint64(n)
	return n, err
}

func (h *sizeHMAC) Sum(in []byte) []byte {
	return h.inner.Sum(in)
}

func (h *sizeHMAC) Reset() {
	h.inner.Reset()
}

func (h *sizeHMAC) MessageSize() uint64 {
	return h.size
}

// HMAC returns the HMAC sum for the given input bytes and HMAC-256 key.
func HMAC(p []byte, hmacKey []byte) ([]byte, error) {
	macer := NewHMAC(hmacKey)
	if _, err := macer.Write(p); err != nil {
		return nil, err
	}
	return macer.Sum(nil), nil
}
