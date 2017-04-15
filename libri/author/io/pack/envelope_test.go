package pack

import (
	"testing"
	"github.com/drausin/libri/libri/author/io/enc"
	"math/rand"
	"github.com/drausin/libri/libri/common/id"
	"github.com/magiconair/properties/assert"
	"github.com/drausin/libri/libri/librarian/api"
)

func TestNewEnvelopeDoc(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	_, authorPub, readerPub := enc.NewPseudoRandomKeys(rng)
	entryKey := id.NewPseudoRandom(rng)
	docEnvelope := NewEnvelopeDoc(authorPub, readerPub, entryKey)
	envelope := docEnvelope.Contents.(*api.Document_Envelope).Envelope
	assert.Equal(t, authorPub, envelope.AuthorPublicKey)
	assert.Equal(t, readerPub, envelope.ReaderPublicKey)
	assert.Equal(t, entryKey.Bytes(), envelope.EntryKey)
}
