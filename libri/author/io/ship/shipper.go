package ship

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/io/pack"
	"errors"
	"bytes"
)

// ErrInconsistentPageKey indicates when a page key does not match the expected value.
var ErrInconsistentPageKey = errors.New("inconsistent page key")

// ErrUnexpectedPage indicates when an entry unexpectedly contains a page instead of page keys.
var ErrUnexpectedPage = errors.New("unexpected entry page")

// Shipper publishes documents to libri.
type Shipper interface {
	// Ship publishes (to libri) the entry document, its page document keys (if more than one),
	// and the envelope document with the author and reader public keys. It returns the
	// published envelope document and entry document key.
	Ship(entry *api.Document, pageKeys []id.ID, authorPub []byte, readerPub []byte) (
		*api.Document, id.ID, error)
}

type shipper struct {
	librarians api.ClientBalancer
	publisher publish.Publisher
	mlPublisher publish.MultiLoadPublisher
}

// NewShipper creates a new Shipper from a librarian api.ClientBalancer and two publisher variants.
func NewShipper(
	librarians api.ClientBalancer,
	publisher publish.Publisher,
	mlPublisher publish.MultiLoadPublisher) Shipper {
	return &shipper{
		librarians: librarians,
		publisher: publisher,
		mlPublisher: mlPublisher,
	}
}

func (s *shipper) Ship(entry *api.Document, pageKeys []id.ID, authorPub []byte, readerPub []byte) (
	*api.Document, id.ID, error) {

	// publish separate pages, if necessary
	if len(pageKeys) > 1 {
		if err := checkConsistentPageKeys(pageKeys, entry); err != nil {
			return nil, nil, err
		}
		if err := s.mlPublisher.Publish(pageKeys, authorPub, s.librarians); err != nil {
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
	if _, err = s.publisher.Publish(envelope, authorPub, lc); err != nil {
		return nil, nil, err
	}

	return envelope, entryKey, nil
}

func checkConsistentPageKeys(expected []id.ID, entry *api.Document) error {
	switch x := entry.Contents.(*api.Document_Entry).Entry.Contents.(type) {
	case *api.Entry_PageKeys:
		for i, keyBytes := range x.PageKeys.Keys {
			if !bytes.Equal(expected[i].Bytes(), keyBytes) {
				return ErrInconsistentPageKey
			}
		}
	case *api.Entry_Page:
		return ErrUnexpectedPage
	}
	return nil
}
