package author

import (
	"github.com/drausin/libri/libri/author/io/print"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"go.uber.org/zap"
	"io"
	"github.com/drausin/libri/libri/author/io/publish"
)

// Author is the main client of the libri network. It can upload, download, and share documents with
// other author clients.
type Author struct {
	// selfID is ID of this author client
	clientID ecid.ID

	// collection of keys for encrypting Envelope documents; these can be used as either the
	// author or reader keys
	authorKeys keychain.Sampler

	// collection of reader keys used with sending Envelope documents to oneself; these are
	// never used as the author key
	selfReaderKeys keychain.Sampler

	// Config holds the configuration parameters of the server
	config *Config

	// key-value store DB used for all external storage
	db db.KVDB

	// load balancer for librarian clients
	librarians api.ClientBalancer

	publisher publish.Publisher

	mlPublisher publish.MultiLoadPublisher

	// SL for locally stored documents
	documentSL storage.DocumentStorerLoader

	// ensures keys are valid
	pageSL page.StorerLoader

	kc storage.Checker

	// ensures keys and values are valid
	kvc storage.KeyValueChecker

	// signs requests
	signer client.Signer

	// logger for this instance
	logger *zap.Logger

	// receives graceful stop signal
	stop chan struct{}
}

// TODO (drausin) Author methods
// - Download()
// - Share()

// Upload compresses, encrypts, and splits the content into pages and then stores them in the
// libri network. It returns the *api.Entry that was stored.
func (a *Author) Upload(content io.Reader, mediaType string) (*api.Entry, error) {
	authorPub, readerPub, keys, err := a.sampleSelfReaderKeys()
	if err != nil {
		return nil, err
	}
	printer := print.NewPrinter(a.config.Print, keys, authorPub, a.pageSL)

	// create entry
	pageKeys, metadata, err := printer.Print(content, mediaType)
	if err != nil {
		return nil, err
	}
	encMetadata, err := enc.EncryptMetadata(metadata, keys)
	if err != nil {
		return nil, err
	}
	entry, multiPage, err := newEntryDoc(authorPub, pageKeys, encMetadata, a.documentSL)
	if err != nil {
		return nil, err
	}

	// publish separate pages (if more than one), entry, and envelope
	if multiPage {
		if err := a.mlPublisher.Publish(pageKeys, a.librarians); err != nil {
			return nil, err
		}
	}
	entryKey, err := a.publisher.Publish(entry, a.librarians.Next())
	if err != nil {
		return nil, err
	}
	envelope := newEnvelopeDoc(authorPub, readerPub, entryKey)
	if _, err = a.publisher.Publish(envelope, a.librarians.Next()); err != nil {
		return nil, err
	}

	// TODO (drausin) store envelope in local storage by entry key

	return entry, nil
}

// sampleSelfReaderKeys samples a random pair of keys (author and reader) for the author to use
// in creating the document *Keys instance. The method returns the author and reader public keys
// along with the *Keys object.
func (a *Author) sampleSelfReaderKeys() ([]byte, []byte, *enc.Keys, error) {
	authorID, err := a.authorKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	selfReaderID, err := a.selfReaderKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	keys, err := enc.NewKeys(authorID.Key(), &selfReaderID.Key().PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return ecid.ToPublicKeyBytes(authorID), ecid.ToPublicKeyBytes(selfReaderID), keys, nil
}

