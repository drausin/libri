package ship

import (
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
)

// Receiver downloads the envelope, entry, and pages from the libri network.
type Receiver interface {
	// Receive gets (from libri) the envelope, entry, and pages implied by the envelope key. It
	// stores these documents in a storage.DocumentStorer and returns the entry and encryption
	// keys.
	Receive(envelopeKey id.ID) (*api.Document, *enc.EEK, error)
}

type receiver struct {
	librarians api.ClientBalancer
	readerKeys keychain.Keychain
	acquirer   publish.Acquirer
	msAcquirer publish.MultiStoreAcquirer
	docS       storage.DocumentStorer
}

// NewReceiver creates a new Receiver from the librarian balancer, keychain of reader keys,
// acquirers, and storage.DocumentStorer.
func NewReceiver(
	librarians api.ClientBalancer,
	readerKeys keychain.Keychain,
	acquirer publish.Acquirer,
	msAcquirer publish.MultiStoreAcquirer,
	docS storage.DocumentStorer,
) Receiver {
	return &receiver{
		librarians: librarians,
		readerKeys: readerKeys,
		acquirer:   acquirer,
		msAcquirer: msAcquirer,
		docS:       docS,
	}
}

func (r *receiver) Receive(envelopeKey id.ID) (*api.Document, *enc.EEK, error) {
	lc, err := r.librarians.Next()
	if err != nil {
		return nil, nil, err
	}

	// get the envelope and encryption keys
	envelopeDoc, err := r.acquirer.Acquire(envelopeKey, nil, lc)
	if err != nil {
		return nil, nil, err
	}
	envelope, ok := envelopeDoc.Contents.(*api.Document_Envelope)
	if !ok {
		return nil, nil, api.ErrUnexpectedDocumentType
	}
	eek, authorPubBytes, err := r.createEEK(envelope.Envelope)
	if err != nil {
		return nil, nil, err
	}

	// get the entry and pages
	entryKey := id.FromBytes(envelope.Envelope.EntryKey)
	entryDoc, err := r.acquirer.Acquire(entryKey, authorPubBytes, lc)
	if err != nil {
		return nil, nil, err
	}
	if err := r.getPages(entryDoc, authorPubBytes); err != nil {
		return nil, nil, err
	}

	return entryDoc, eek, nil
}

func (r *receiver) createEEK(envelope *api.Envelope) (*enc.EEK, []byte, error) {
	readerPriv, in := r.readerKeys.Get(envelope.ReaderPublicKey)
	if !in {
		return nil, nil, keychain.ErrUnexpectedMissingKey
	}
	authorPub, err := ecid.FromPublicKeyBytes(envelope.AuthorPublicKey)
	if err != nil {
		return nil, nil, err
	}
	kek, err := enc.NewKEK(readerPriv.Key(), authorPub)
	if err != nil {
		return nil, nil, err
	}
	eek, err := kek.Decrypt(envelope.EekCiphertext, envelope.EekCiphertextMac)
	return eek, envelope.AuthorPublicKey, err
}

func (r *receiver) getPages(entry *api.Document, authorPubBytes []byte) error {
	if _, ok := entry.Contents.(*api.Document_Entry); !ok {
		return api.ErrUnexpectedDocumentType
	}
	switch ec := entry.Contents.(*api.Document_Entry).Entry.Contents.(type) {
	case *api.Entry_PageKeys:
		pageKeys, err := api.GetEntryPageKeys(entry)
		if err != nil {
			// should never get here
			return err
		}
		return r.msAcquirer.Acquire(pageKeys, authorPubBytes, r.librarians)
	case *api.Entry_Page:
		pageDoc, docKey, err := api.GetPageDocument(ec.Page)
		if err != nil {
			// should never get here
			return err
		}
		return r.docS.Store(docKey, pageDoc)
	}

	// should never get here
	return api.ErrUnknownDocumentType
}
