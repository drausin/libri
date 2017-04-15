package pack

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// NewEnvelopeDoc returns a new envelope document for the given entry key and author and reader
// public keys.
func NewEnvelopeDoc(authorPub []byte, readerPub []byte, entryKey id.ID) *api.Document {
	envelope := &api.Envelope{
		AuthorPublicKey: authorPub,
		ReaderPublicKey: readerPub,
		EntryKey:        entryKey.Bytes(),
	}
	return &api.Document{
		Contents: &api.Document_Envelope{
			Envelope: envelope,
		},
	}
}
