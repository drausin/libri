package ship

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/keychain"
	"errors"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/storage"
)

type Receiver interface {
	Receive(envelopeKey id.ID) (*api.Document, *enc.Keys, error)
}


type receiver struct {
	librarians api.ClientBalancer
	readerKeys keychain.Keychain
	acquirer publish.Acquirer
	msAcquirer publish.MultiStoreAcquirer
	docS storage.DocumentStorer
}

func (r *receiver) Receive(envelopeKey id.ID) (*api.Document, *enc.Keys, error) {
	lc, err := r.librarians.Next()
	if err != nil {
		return nil, nil, err
	}

	// get the envelop and encryption keys
	envelopeDoc, err := r.acquirer.Acquire(envelopeKey, nil, lc)
	if err != nil {
		return nil, nil, err
	}
	authorPubBytes, readerPubBytes, entryKey, err := pack.SeparateEnvelopeDoc(envelopeDoc)
	if err != nil {
		return nil, nil, err
	}
	encKeys, err := r.createEncryptionKeys(readerPubBytes)
	if err != nil {
		return nil, nil, err
	}

	// get the entry and pages
	entryDoc, err := r.acquirer.Acquire(entryKey, authorPubBytes, lc)
	if err != nil {
		return nil, nil, err
	}
	if err := r.getPages(entryDoc, authorPubBytes); err != nil {
		return nil, nil, err
	}

	return entryDoc, encKeys, nil
}

func (r *receiver) createEncryptionKeys(readerPubBytes []byte) (*enc.Keys, error) {
	readerPriv, in := r.readerKeys.Get(readerPubBytes)
	if !in {
		return nil, errors.New("unable to find envelope reader key")
	}
	readerPub, err := ecid.FromPublicKeyBytes(readerPubBytes)
	if err != nil {
		return nil, err
	}
	encKeys, err := enc.NewKeys(readerPriv.Key(), readerPub)
	if err != nil {
		return nil, err
	}
	return encKeys, nil
}

func (r *receiver) getPages(entry *api.Document, authorPubBytes []byte) error {
	if _, ok := entry.Contents.(*api.Document_Entry); !ok {
		return api.ErrUnexpectedDocumentType
	}
	switch ec := entry.Contents.(*api.Document_Entry).Entry.Contents.(type) {
	case *api.Entry_PageKeys:
		pageKeys, err := api.GetEntryPageKeys(entry)
		if err != nil {
			return err
		}
		return r.msAcquirer.Acquire(pageKeys, authorPubBytes, r.librarians)
	case *api.Entry_Page:
		pageDoc, docKey, err := api.GetPageDocument(ec.Page)
		if err != nil {
			return err
		}
		return r.docS.Store(docKey, pageDoc)
	}

	return api.ErrUnknownDocumentType
}