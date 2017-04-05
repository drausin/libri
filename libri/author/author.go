package author

import (
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/db"
	"go.uber.org/zap"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/author/keychain"
	"io"
	"github.com/drausin/libri/libri/author/io/encryption"
	"github.com/drausin/libri/libri/author/io/compression"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/author/io/pagination"
	"github.com/drausin/libri/libri/common/id"
	"errors"
	"time"
	"sync"
)

const (
	DefaultPutQueryTimeout = 5 * time.Second
)

// ErrUnexpectedMissingDocument indicates when a document is unexpectely missing from the document
// storer loader.
var ErrUnexpectedMissingDocument = errors.New("unexpected missing document")

type Author struct {
	// selfID is ID of this author client
	clientID   ecid.ID

	// collection of keys for encrypting Envelope documents; these can be used as either the
	// author or reader keys
	authorKeys keychain.Sampler

	// collection of reader keys used with sending Envelope documents to oneself; these are
	// never used as the author key
	selfReaderKeys keychain.Sampler

	// Config holds the configuration parameters of the server
	config     *Config

	// key-value store DB used for all external storage
	db         db.KVDB

	// load balancer for librarian clients
	librarians LibrarianBalancer

	// SL for locally stored documents
	documentSL storage.DocumentStorerLoader

	pageSL     pagination.StorerLoader

	// ensures keys are valid
	kc         storage.Checker

	// ensures keys and values are valid
	kvc        storage.KeyValueChecker

	// signs requests
	signer     client.Signer

	// logger for this instance
	logger     *zap.Logger

	// receives graceful stop signal
	stop       chan struct{}
}

// TODO (drausin) Author methods
// - Download()
// - Share()

func (a *Author) Upload(content io.Reader, mediaType string) error {
	authorPub, readerPub, keys, err := a.sampleAuthorSelfReaderKeys()
	if err != nil {
		return err
	}
	codec, err := compression.GetCompressionCodec(mediaType)
	if err != nil {
		return err
	}
	compressor, err := compression.NewCompressor(content, codec, compression.DefaultBufferSize)
	if err != nil {
		return err
	}
	encrypter, err := encryption.NewEncrypter(keys)
	if err != nil {
		return err
	}

	pages := make(chan *api.Page, 3)  // TODO (drausin) configure this
	paginator, err := pagination.NewPaginator(pages, encrypter, keys, authorPub,
		pagination.DefaultSize)
	if err != nil {
		return err
	}

	go func() {
		_, err = paginator.ReadFrom(compressor)
		if err != nil {
			return err
		}
		close(pages)
	}()

	pageIDs, err := a.pageSL.Store(pages)
	if err != nil {
		return err
	}
	metadata, err := api.NewEntryMetadata(
		mediaType,
		paginator.CiphertextMAC().MessageSize(),
		paginator.CiphertextMAC().Sum(nil),
		// TODO (drausin) update these
		0,
		[]byte{},
	)
	if err != nil {
		return err
	}
	encMetadata, err := encryption.EncryptMetadata(metadata, keys)
	if err != nil {
		return err
	}
	entry, err := a.newEntry(authorID, pageIDs, encMetadata)
	if err != nil {
		return err
	}
	if _, ok := entry.Contents.(*api.PageKeys); ok {
		pagePutParallelism := 3 // TODO (drausin) configure parallelism
		putPageIDs := make(chan id.ID, pagePutParallelism)
		go func() {
			for _, pageID := range pageIDs {
				putPageIDs <- pageID
			}
		}()
		wg := new(sync.WaitGroup)
		putErrs := make(chan error, 1)
		for c := 0; c < pagePutParallelism; c++ {
			wg.Add(1)
			go func() {
				if err := a.putPages(putPageIDs); err != nil {
					putErrs <- err
				}

			}()
		}
		close(putErrs)
		close(putPageIDs)
		wg.Wait()
		select {
		case err <- putErrs:
			return err
		}
	}
	entryKey, err := a.put(entry, a.librarians.Next())
	if err != nil {
		return err
	}

	envelope := api.Envelope{
		AuthorPublicKey: authorPub,
		ReaderPublicKey: readerPub,
		EntryKey: entryKey.Bytes(),
	}
	_, err = a.put(envelope, a.librarians.Next())
	if err != nil {
		return err
	}

	return nil
}

// sampleKeys samples a random pair of keys (author and reader) for the author to use in creating
// the document *Keys instance. The method returns the author and reader public keys along with the
// *Keys object.
func (a *Author) sampleAuthorSelfReaderKeys() ([]byte, []byte, *encryption.Keys, error) {
	authorID, err := a.authorKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	selfReaderID, err := a.selfReaderKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	keys, err := encryption.NewKeys(authorID.Key(), &selfReaderID.Key().PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return ecid.ToPublicKeyBytes(authorID), ecid.ToPublicKeyBytes(selfReaderID), keys, nil
}

func (a *Author) put(doc *api.Document, lClient api.LibrarianClient) (id.ID, error) {
	docKey, err := api.GetKey(a)
	if err != nil {
		return nil, err
	}
	rq := client.NewPutRequest(a.clientID, docKey, doc)
	// TODO (drausin) configure timeout
	ctx, cancel, err := client.NewSignedTimeoutContext(a.signer, rq, DefaultPutQueryTimeout)
	if err != nil {
		return nil, err
	}
	_, err = lClient.Put(ctx, rq)
	cancel()
	if err != nil {
		return nil, err
	}
	return docKey, nil
}

func (a *Author) putPages(pageIDs chan id.ID) error {
	for pageID := range pageIDs {
		pageDoc, err := a.documentSL.Load(pageID)
		if err != nil {
			return err
		}
		if pageDoc == nil {
			return ErrUnexpectedMissingDocument
		}
		if _, err := a.put(pageDoc, a.librarians.Next()); err != nil {
			return err
		}
	}
	return nil
}

func (a *Author) newEntry(
	authorID ecid.ID, pageIDs []id.ID, encMetadata *encryption.EncryptedMetadata,
) (*api.Entry, error) {
	if len(pageIDs) == 1 {
		return a.newSinglePageEntry(authorID, pageIDs[0], encMetadata)
	}
	return a.newMultiPageEntry(authorID, pageIDs, encMetadata)
}

func (a *Author) newSinglePageEntry(
	authorID ecid.ID, pageID id.ID, encMetadata *encryption.EncryptedMetadata,
) (*api.Entry, error) {
	pageDoc, err := a.documentSL.Load(pageID)
	if err != nil {
		return nil, err
	}
	pageContent, ok := pageDoc.Contents.(*api.Document_Page)
	if !ok {
		return nil, errors.New("not a page")
	}
	return &api.Entry{
		AuthorPublicKey: ecid.ToPublicKeyBytes(authorID),
		Contents: &api.Entry_Page{
			Page: pageContent.Page,
		},
		CreatedTime: time.Now().Unix(),
		MetadataCiphertext: encMetadata.Ciphertext,
		MetadataCiphertextMac: encMetadata.CiphertextMAC,
	}, nil
}

func (a *Author) newMultiPageEntry(
	authorID ecid.ID, pageIDs []id.ID, encMetadata *encryption.EncryptedMetadata,
) (*api.Entry, error) {

	pageKeys := make([][]byte, len(pageIDs))
	for i, pageID := range pageIDs {
		pageKeys[i] = pageID.Bytes()
	}

	return &api.Entry{
		AuthorPublicKey: ecid.ToPublicKeyBytes(authorID),
		Contents: &api.Entry_PageKeys{
			PageKeys: &api.PageKeys{Keys: pageKeys},
		},
		CreatedTime: time.Now().Unix(),
		MetadataCiphertext: encMetadata.Ciphertext,
		MetadataCiphertextMac: encMetadata.CiphertextMAC,
	}, nil
}
