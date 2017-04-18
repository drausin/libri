package pack

import (
	"io"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/author/io/print"
	"github.com/drausin/libri/libri/author/io/page"
	"time"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"errors"
)

// EntryPacker creates entry documents from raw content.
type EntryPacker interface {
	// Pack prints pages from the content, encrypts their metadata, and binds them together
	// into an entry *api.Document and list of page keys.
	Pack(content io.Reader, mediaType string, keys *enc.Keys, authorPub []byte) (
		*api.Document, []id.ID, error)
}

// NewEntryPacker creates a new Packer instance.
func NewEntryPacker(
	params *print.Parameters,
	metadataEnc enc.MetadataEncrypter,
	docSL storage.DocumentStorerLoader,
) EntryPacker {
	return &entryPacker{
		params: params,
		metadataEnc: metadataEnc,
		pageS: page.NewStorerLoader(docSL),
		docL: docSL,
	}
}

type entryPacker struct {
	params *print.Parameters
	metadataEnc enc.MetadataEncrypter
	pageS page.Storer
	docL storage.DocumentLoader
}

func (p *entryPacker) Pack(content io.Reader, mediaType string, keys *enc.Keys, authorPub []byte) (
	*api.Document, []id.ID, error) {

	printer := print.NewPrinter(p.params, keys, authorPub, p.pageS)
	pageKeys, metadata, err := printer.Print(content, mediaType)
	if err != nil {
		return nil, nil, err
	}
	encMetadata, err := p.metadataEnc.Encrypt(metadata, keys)
	if err != nil {
		return nil, nil, err
	}
	doc, err := newEntryDoc(authorPub, pageKeys, encMetadata, p.docL)
	return doc, pageKeys, err
}

type EntryUnpacker interface {
	Unpack(entry *api.Document, pageKeys []id.ID, keys *enc.Keys, content io.Writer) error
}

type entryUnpacker struct {
	params *print.Parameters
	metadataDec enc.MetadataDecrypter
	pageL page.Loader
	docS storage.DocumentStorer
}


func newEntryDoc(
	authorPub []byte,
	pageIDs []id.ID,
	encMeta *enc.EncryptedMetadata,
	docL storage.DocumentLoader,
) (*api.Document, error) {

	var entry *api.Entry
	var err error
	if len(pageIDs) == 1 {
		entry, err = newSinglePageEntry(authorPub, pageIDs[0], encMeta, docL)
	} else {
		entry, err = newMultiPageEntry(authorPub, pageIDs, encMeta)
	}
	if err != nil {
		return nil, err
	}

	doc := &api.Document{
		Contents: &api.Document_Entry{
			Entry: entry,
		},
	}
	return doc, nil
}

func newSinglePageEntry(
	authorPub []byte,
	pageKey id.ID,
	encMeta *enc.EncryptedMetadata,
	docL storage.DocumentLoader,
) (*api.Entry, error) {

	pageDoc, err := docL.Load(pageKey)
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
