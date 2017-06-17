package pack

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestNewEnvelopeDoc(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	_, authorPub, readerPub := enc.NewPseudoRandomKEK(rng)
	ciphertext := api.RandBytes(rng, api.EEKLength)
	ciphertextMAC := api.RandBytes(rng, api.HMAC256Length)

	entryKey := id.NewPseudoRandom(rng)
	docEnvelope := NewEnvelopeDoc(entryKey, authorPub, readerPub, ciphertext, ciphertextMAC)
	envelope := docEnvelope.Contents.(*api.Document_Envelope).Envelope
	assert.Equal(t, authorPub, envelope.AuthorPublicKey)
	assert.Equal(t, readerPub, envelope.ReaderPublicKey)
	assert.Equal(t, entryKey.Bytes(), envelope.EntryKey)
	assert.Equal(t, ciphertext, envelope.EekCiphertext)
	assert.Equal(t, ciphertextMAC, envelope.EekCiphertextMac)
}
