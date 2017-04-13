package author

import (
	"io"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"go.uber.org/zap"
)

// Author is the main client of the libri network. It can upload, download, and share documents with
// other author clients.
type Author struct {
	// selfID is ID of this author client
	clientID ecid.ID

	// Config holds the configuration parameters of the server
	config *Config

	// collection of keys for encrypting Envelope documents; these can be used as either the
	// author or reader keys
	authorKeys keychain.Keychain

	// collection of reader keys used with sending Envelope documents to oneself; these are
	// never used as the author key
	selfReaderKeys keychain.Keychain

	// samples a pair of author and selfReader keys for encrypting an entry
	envelopeKeys envelopeKeySampler

	// key-value store DB used for all external storage
	db db.KVDB

	// SL for client data
	clientSL storage.NamespaceStorerLoader

	// SL for locally stored documents
	documentSL storage.DocumentLoader

	// load balancer for librarian clients
	librarians api.ClientBalancer

	// publishes documents to libri network
	publisher publish.Publisher

	// loads and publishes documents in parallel to libri network
	mlPublisher publish.MultiLoadPublisher

	// creates entry documents from raw content
	entryPacker pack.EntryPacker

	// stores Pages in chan to local storage
	pageSL page.StorerLoader

	// signs requests
	signer client.Signer

	// logger for this instance
	logger *zap.Logger

	// receives graceful stop signal
	stop chan struct{}
}

// NewAuthor creates a new *Author from the Config, decrypting the keychains with the supplied
// auth string.
func NewAuthor(config *Config, keychainAuth string, logger *zap.Logger) (*Author, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		logger.Error("unable to init RocksDB", zap.Error(err))
		return nil, err
	}
	clientSL := storage.NewClientKVDBStorerLoader(rdb)
	documentSL := storage.NewDocumentKVDBStorerLoader(rdb)

	// get client ID and immediately save it so subsequent restarts have it
	clientID, err := loadOrCreateClientID(logger, clientSL)
	if err != nil {
		return nil, err
	}

	authorKeys, selfReaderKeys, err := loadKeychains(config.KeychainDir, keychainAuth)
	if err != nil {
		return nil, err
	}
	envelopeKeys := &envelopeKeySamplerImpl{
		authorKeys:     authorKeys,
		selfReaderKeys: selfReaderKeys,
	}
	librarians, err := api.NewUniformRandomClientBalancer(config.LibrarianAddrs)
	if err != nil {
		return nil, err
	}
	signer := client.NewSigner(clientID.Key())
	publisher := publish.NewPublisher(clientID, signer, config.Publish)
	slPublisher := publish.NewSingleLoadPublisher(publisher, documentSL)
	entryPacker := pack.NewEntryPacker(
		config.Print,
		enc.NewMetadataEncrypterDecrypter(),
		documentSL,
	)

	return &Author{
		clientID:       clientID,
		config:         config,
		authorKeys:     authorKeys,
		selfReaderKeys: selfReaderKeys,
		envelopeKeys:   envelopeKeys,
		db:             rdb,
		clientSL:       clientSL,
		documentSL:     documentSL,
		librarians:     librarians,
		publisher:      publish.NewPublisher(clientID, signer, config.Publish),
		mlPublisher:    publish.NewMultiLoadPublisher(slPublisher, config.Publish),
		entryPacker:    entryPacker,
		pageSL:         page.NewStorerLoader(documentSL),
		signer:         signer,
		logger:         logger,
		stop:           make(chan struct{}),
	}, nil
}

// TODO (drausin) Author methods
// - Download()
// - Share()

// Upload compresses, encrypts, and splits the content into pages and then stores them in the
// libri network. It returns the *api.Entry that was stored.
func (a *Author) Upload(content io.Reader, mediaType string) (*api.Document, error) {
	authorPub, readerPub, keys, err := a.envelopeKeys.sample()
	if err != nil {
		return nil, err
	}
	entry, pageKeys, err := a.entryPacker.Pack(content, mediaType, keys, authorPub)
	if err != nil {
		return nil, err
	}

	// publish separate pages (if more than one), entry, and envelope
	if len(pageKeys) > 1 {
		if err := a.mlPublisher.Publish(pageKeys, a.librarians); err != nil {
			return nil, err
		}
	}
	lc, err := a.librarians.Next()
	if err != nil {
		return nil, err
	}
	entryKey, err := a.publisher.Publish(entry, lc)
	if err != nil {
		return nil, err
	}
	envelope := pack.NewEnvelopeDoc(authorPub, readerPub, entryKey)
	if _, err = a.publisher.Publish(envelope, lc); err != nil {
		return nil, err
	}

	// TODO (drausin)
	// - store envelope in local storage by entry key
	// - delete pages from local storage

	return entry, nil
}
