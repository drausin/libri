package encryption

import (
	"hash"
	"crypto/hmac"
	"crypto/sha256"
	"io"
)

type MACer interface {
	io.Writer
	Sum(in []byte) []byte
	Reset()
	MessageSize() uint64
}

type hmacer struct {
	inner hash.Hash
	size  uint64
}

func NewHMACer(hmacKey []byte) MACer {
	return &hmacer{
		inner: hmac.New(sha256.New, hmacKey),
	}
}

func (h *hmacer) Write(p []byte) (int, error) {
	n, err := h.inner.Write(p)
	h.size += uint64(n)
	return n, err
}

func (h *hmacer) Sum(in []byte) []byte {
	if in != nil {
		h.size += uint64(len(in))
	}
	return h.inner.Sum(in)
}

func (h *hmacer) Reset() {
	h.inner.Reset()
}

func (h *hmacer) MessageSize() uint64 {
	return h.size
}

func HMAC(in []byte, hmacKey []byte) []byte {
	macer := NewHMACer(hmacKey)
	return macer.Sum(in)
}
