package ship

import (
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
)

// Shipper publishes documents to libri.
type Shipper interface {
	// ShipEntry publishes (to libri) the entry document, its page document keys (if more than one),
	// and the envelope document with the author and reader public keys. It returns the
	// published envelope document and its key.
	ShipEntry(
		entry *api.Document, authorPub []byte, readerPub []byte, kek *enc.KEK, eek *enc.EEK,
	) (*api.Document, id.ID, error)

	ShipEnvelope(entryKey id.ID, authorPub, readerPub []byte, kek *enc.KEK, eek *enc.EEK) (
		*api.Document, id.ID, error)
}

type shipper struct {
	librarians  client.PutterBalancer
	publisher   publish.Publisher
	mlPublisher publish.MultiLoadPublisher
	deletePages bool
}

// NewShipper creates a new Shipper from a librarian api.Balancer and two publisher variants.
func NewShipper(
	librarians client.PutterBalancer,
	publisher publish.Publisher,
	mlPublisher publish.MultiLoadPublisher) Shipper {
	return &shipper{
		librarians:  librarians,
		publisher:   publisher,
		mlPublisher: mlPublisher,
		deletePages: true,
	}
}

func (s *shipper) ShipEntry(
	entry *api.Document, authorPub []byte, readerPub []byte, kek *enc.KEK, eek *enc.EEK,
) (*api.Document, id.ID, error) {

	// publish separate pages, if necessary
	pageKeys, err := api.GetEntryPageKeys(entry)
	if err != nil {
		return nil, nil, err
	}
	if pageKeys != nil {
		err = s.mlPublisher.Publish(pageKeys, authorPub, s.librarians, s.deletePages)
		if err != nil {
			return nil, nil, err
		}
	}

	rlc := s.mlPublisher.GetRetryPutter(s.librarians)
	entryKey, err := s.publisher.Publish(entry, authorPub, rlc)
	if err != nil {
		return nil, nil, err
	}
	return s.ShipEnvelope(entryKey, authorPub, readerPub, kek, eek)
}

func (s *shipper) ShipEnvelope(
	entryKey id.ID, authorPub, readerPub []byte, kek *enc.KEK, eek *enc.EEK,
) (*api.Document, id.ID, error) {

	eekCiphertext, eekCiphertextMAC, err := kek.Encrypt(eek)
	if err != nil {
		return nil, nil, err
	}
	envelope := pack.NewEnvelopeDoc(entryKey, authorPub, readerPub, eekCiphertext, eekCiphertextMAC)
	rlc := s.mlPublisher.GetRetryPutter(s.librarians)
	envelopeKey, err := s.publisher.Publish(envelope, authorPub, rlc)
	if err != nil {
		return nil, nil, err
	}
	return envelope, envelopeKey, nil
}
