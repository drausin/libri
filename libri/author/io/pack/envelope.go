package pack

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

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
