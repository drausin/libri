package ship

import (
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
)

// Receiver downloads the envelope, entry, and pages from the libri network.
type Receiver interface {
	// ReceiveEntry gets (from libri) the envelope, entry, and pages implied by the envelope key. It
	// stores these documents in a storage.DocumentStorer and returns the entry and encryption
	// keys.
	ReceiveEntry(envelopeKey id.ID) (*api.Document, *enc.EEK, error)

	ReceiveEnvelope(envelopeKey id.ID) (*api.Envelope, error)

	GetEEK(envelope *api.Envelope) (*enc.EEK, error)
}

type receiver struct {
	librarians client.GetterBalancer
	readerKeys keychain.Getter
	acquirer   publish.Acquirer
	msAcquirer publish.MultiStoreAcquirer
	docS       storage.DocumentStorer
}

// NewReceiver creates a new Receiver from the librarian balancer, keychain of reader keys,
// acquirers, and storage.DocumentStorer.
func NewReceiver(
	librarians client.GetterBalancer,
	readerKeys keychain.Getter,
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

func (r *receiver) ReceiveEntry(envelopeKey id.ID) (*api.Document, *enc.EEK, error) {
	envelope, err := r.ReceiveEnvelope(envelopeKey)
	if err != nil {
		return nil, nil, err
	}
	eek, err := r.GetEEK(envelope)
	if err != nil {
		return nil, nil, err
	}
	// get the entry and pages
	entryKey := id.FromBytes(envelope.EntryKey)
	rlc := r.msAcquirer.GetRetryGetter(r.librarians)
	entryDoc, err := r.acquirer.Acquire(entryKey, envelope.AuthorPublicKey, rlc)
	if err != nil {
		return nil, nil, err
	}
	if err := r.getPages(entryDoc, envelope.AuthorPublicKey); err != nil {
		return nil, nil, err
	}
	return entryDoc, eek, nil
}

func (r *receiver) ReceiveEnvelope(envelopeKey id.ID) (*api.Envelope, error) {
	rlc := r.msAcquirer.GetRetryGetter(r.librarians)
	envelopeDoc, err := r.acquirer.Acquire(envelopeKey, nil, rlc)
	if err != nil {
		return nil, err
	}
	envelope, ok := envelopeDoc.Contents.(*api.Document_Envelope)
	if !ok {
		return nil, api.ErrUnexpectedDocumentType
	}
	return envelope.Envelope, nil
}

func (r *receiver) GetEEK(envelope *api.Envelope) (*enc.EEK, error) {
	readerPriv, in := r.readerKeys.Get(envelope.ReaderPublicKey)
	if !in {
		return nil, keychain.ErrUnexpectedMissingKey
	}
	authorPub, err := ecid.FromPublicKeyBytes(envelope.AuthorPublicKey)
	if err != nil {
		return nil, err
	}
	kek, err := enc.NewKEK(readerPriv.Key(), authorPub)
	if err != nil {
		return nil, err
	}
	eek, err := kek.Decrypt(envelope.EekCiphertext, envelope.EekCiphertextMac)
	return eek, err
}

func (r *receiver) getPages(entryDoc *api.Document, authorPubBytes []byte) error {
	entry, ok := entryDoc.Contents.(*api.Document_Entry)
	if !ok {
		return api.ErrUnexpectedDocumentType
	}
	if entry.Entry.Page != nil {
		pageDoc, docKey, err := api.GetPageDocument(entry.Entry.Page)
		errors.MaybePanic(err) // should never happen
		return r.docS.Store(docKey, pageDoc)
	}
	if entry.Entry.PageKeys != nil {
		pageKeys, err := api.GetEntryPageKeys(entryDoc)
		errors.MaybePanic(err) // should never happen
		return r.msAcquirer.Acquire(pageKeys, authorPubBytes, r.librarians)
	}

	// should never get here
	return api.ErrUnknownDocumentType
}
