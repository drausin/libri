package pack

import (
	"testing"
	"github.com/drausin/libri/libri/author/io/enc"
	"math/rand"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
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

func TestSeparateEnvelopeDoc(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	_, authorPub1, readerPub1 := enc.NewPseudoRandomKeys(rng)
	entryKey1 := id.NewPseudoRandom(rng)
	docEnvelope := NewEnvelopeDoc(authorPub1, readerPub1, entryKey1)

	authorPub2, readerPub2, entryKey2, err := SeparateEnvelopeDoc(docEnvelope)
	assert.Nil(t, err)
	assert.Equal(t, authorPub1, authorPub2)
	assert.Equal(t, readerPub1, readerPub2)
	assert.Equal(t, entryKey1, entryKey2)

	// check it errors on non-envelope doc
	entryDoc, _ := api.NewTestDocument(rng)
	authorPub3, readerPub3, entryKey3, err := SeparateEnvelopeDoc(entryDoc)
	assert.NotNil(t, err)
	assert.Nil(t, authorPub3)
	assert.Nil(t, readerPub3)
	assert.Nil(t, entryKey3)
}
