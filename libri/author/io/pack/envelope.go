package pack

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
)

// NewEnvelopeDoc returns a new envelope document for the given entry key and author and reader
// public keys.
func NewEnvelopeDoc(authorPub, readerPub []byte, entryKey id.ID) *api.Document {
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

// SeparateEnvelopeDoc returns the author and reader public keys along with the entry ID in an
// envelope.
func SeparateEnvelopeDoc(envelope *api.Document) ([]byte, []byte, id.ID, error) {
	switch c := envelope.Contents.(type) {
	case *api.Document_Envelope:
		e := c.Envelope
		return e.AuthorPublicKey, e.ReaderPublicKey, id.FromBytes(e.EntryKey)
	}
	return api.ErrUnexpectedDocumentType
}
