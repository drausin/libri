package ship

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/pkg/errors"
	"github.com/drausin/libri/libri/common/ecid"
)

type Receiver interface {
	Receive(envelopeKey id.ID) (*api.Document, []id.ID, *enc.Keys, error)
}


type receiver struct {
	librarians api.ClientBalancer
	readerKeys keychain.Keychain
	acquirer publish.Acquirer
	msAcquirer publish.MultiStoreAcquirer
}

func (r *receiver) Receive(envelopeKey id.ID) (*api.Document, []id.ID, *enc.Keys, error) {
	lc, err := r.librarians.Next()
	if err != nil {
		return nil, nil, nil, err
	}

	// get the envelop and encryption keys
	envelopeDoc, err := r.acquirer.Acquire(envelopeKey, nil, lc)
	if err != nil {
		return nil, nil, nil, err
	}
	authorPubBytes, readerPubBytes, entryKey, err := pack.SeparateEnvelopeDoc(envelopeDoc)
	if err != nil {
		return nil, nil, nil, err
	}
	encKeys, err := r.createEncryptionKeys(readerPubBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	// get the entry and pages
	entryDoc, err := r.acquirer.Acquire(entryKey, authorPubBytes, lc)
	if err != nil {
		return nil, nil, nil, err
	}
	pageKeys, err := r.getPages(entryDoc, authorPubBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	return entryDoc, pageKeys, encKeys, nil
}

func (r *receiver) createEncryptionKeys(readerPubBytes []byte) (*enc.Keys, error) {
	readerPriv, in := r.readerKeys.Get(readerPubBytes)
	if !in {
		return nil, nil, nil, errors.New("unable to find envelope reader key")
	}
	readerPub, err := ecid.FromPublicKeyBytes(readerPubBytes)
	if err != nil {
		return nil, nil, nil, err
	}
	encKeys, err := enc.NewKeys(readerPriv.Key(), readerPub)
	if err != nil {
		return nil, nil, nil, err
	}
	return encKeys, nil
}

func (r *receiver) getPages(entry *api.Document, authorPubBytes []byte) ([]id.ID, error) {
	switch dc := entry.Contents.(type) {
	case *api.Document_Entry:
		switch ec := dc.Entry.Contents.(type) {
		case *api.Entry_PageKeys:
			pageKeys := make([]id.ID, len(ec.PageKeys.Keys))
			for i, pageKeyBytes := range ec.PageKeys.Keys {
				pageKeys[i] = id.FromBytes(pageKeyBytes)
			}
			err := r.msAcquirer.Acquire(pageKeys, authorPubBytes, r.librarians)
			if err != nil {
				return nil, err
			}
		case *api.Entry_Page:
			return nil, nil
		}
	}
	return nil, api.ErrUnexpectedDocumentType
}