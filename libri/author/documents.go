package author

import (
	"errors"
	"time"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
)

func newEntryDoc(
	authorPub []byte,
	pageIDs []id.ID,
	encMeta *enc.EncryptedMetadata,
	docSL storage.DocumentStorerLoader,
) (*api.Document, bool, error) {

	var entry *api.Entry
	var err error
	isMultiPage := false
	if len(pageIDs) == 1 {
		isMultiPage = true
		entry, err = newSinglePageEntry(authorPub, pageIDs[0], encMeta, docSL)
	} else {
		entry, err = newMultiPageEntry(authorPub, pageIDs, encMeta)
	}

	doc := &api.Document{
		Contents: &api.Document_Entry{
			Entry: entry,
		},
	}
	return doc, isMultiPage, err
}

func newSinglePageEntry(
	authorPub []byte,
	pageKey id.ID,
	encMeta *enc.EncryptedMetadata,
	docSL storage.DocumentStorerLoader,
) (*api.Entry, error) {

	pageDoc, err := docSL.Load(pageKey)
	if err != nil {
		return nil, err
	}
	pageContent, ok := pageDoc.Contents.(*api.Document_Page)
	if !ok {
		return nil, errors.New("not a page")
	}
	return &api.Entry{
		AuthorPublicKey: authorPub,
		Contents: &api.Entry_Page{
			Page: pageContent.Page,
		},
		CreatedTime:           time.Now().Unix(),
		MetadataCiphertext:    encMeta.Ciphertext,
		MetadataCiphertextMac: encMeta.CiphertextMAC,
	}, nil
}

func newMultiPageEntry(
	authorPub []byte, pageKeys []id.ID, encMeta *enc.EncryptedMetadata,
) (*api.Entry, error) {

	pageKeyBytes := make([][]byte, len(pageKeys))
	for i, pageKey := range pageKeys {
		pageKeyBytes[i] = pageKey.Bytes()
	}

	return &api.Entry{
		AuthorPublicKey: authorPub,
		Contents: &api.Entry_PageKeys{
			PageKeys: &api.PageKeys{Keys: pageKeyBytes},
		},
		CreatedTime:           time.Now().Unix(),
		MetadataCiphertext:    encMeta.Ciphertext,
		MetadataCiphertextMac: encMeta.CiphertextMAC,
	}, nil
}

func newEnvelopeDoc(authorPub []byte, readerPub []byte, entryKey id.ID) *api.Document {
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
