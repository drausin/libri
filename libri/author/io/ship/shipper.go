package ship

import (
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// Shipper publishes documents to libri.
type Shipper interface {
	// Ship publishes (to libri) the entry document, its page document keys (if more than one),
	// and the envelope document with the author and reader public keys. It returns the
	// published envelope document and its key.
	Ship(entry *api.Document, authorPub []byte, readerPub []byte) (*api.Document, id.ID, error)
}

type shipper struct {
	librarians  api.ClientBalancer
	publisher   publish.Publisher
	mlPublisher publish.MultiLoadPublisher
}

// NewShipper creates a new Shipper from a librarian api.ClientBalancer and two publisher variants.
func NewShipper(
	librarians api.ClientBalancer,
	publisher publish.Publisher,
	mlPublisher publish.MultiLoadPublisher) Shipper {
	return &shipper{
		librarians:  librarians,
		publisher:   publisher,
		mlPublisher: mlPublisher,
	}
}

func (s *shipper) Ship(entry *api.Document, authorPub []byte, readerPub []byte) (
	*api.Document, id.ID, error) {

	// publish separate pages, if necessary
	pageKeys, err := api.GetEntryPageKeys(entry)
	if err != nil {
		return nil, nil, err
	}
	if pageKeys != nil {
		if err = s.mlPublisher.Publish(pageKeys, authorPub, s.librarians); err != nil {
			return nil, nil, err
		}
	}

	// use same librarian to publish entry and envelope
	lc, err := s.librarians.Next()
	if err != nil {
		return nil, nil, err
	}
	entryKey, err := s.publisher.Publish(entry, authorPub, lc)
	if err != nil {
		return nil, nil, err
	}
	envelope := pack.NewEnvelopeDoc(authorPub, readerPub, entryKey)
	envelopeKey, err := s.publisher.Publish(envelope, authorPub, lc)
	if err != nil {
		return nil, nil, err
	}

	return envelope, envelopeKey, nil
}
