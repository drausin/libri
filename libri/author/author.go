package author

import (
	"io"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/io/ship"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"go.uber.org/zap"
	"fmt"
)

const (
	// LoggerEntryKey is the logger key used for the key of an Entry document.
	LoggerEntryKey = "entry_key"

	// LoggerEnvelopeKey is the logger key used for the key of an Envelope document.
	LoggerEnvelopeKey = "envelope_key"

	// LoggerAuthorPub is the logger key used for an author public key.
	LoggerAuthorPub = "author_pub"

	// LoggerReaderPub is the logger key used for a reader public key.
	LoggerReaderPub = "reader_pub"

	// LoggerNPages is the logger key used for the number of pages in a document.
	LoggerNPages = "n_pages"
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
	documentSL storage.DocumentStorerLoader

	// load balancer for librarian clients
	librarians api.ClientBalancer

	// creates entry documents from raw content
	entryPacker pack.EntryPacker

	entryUnpacker pack.EntryUnpacker

	// publishes documents to libri
	shipper ship.Shipper

	receiver ship.Receiver

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
	acquirer := publish.NewAcquirer(clientID, signer, config.Publish)
	slPublisher := publish.NewSingleLoadPublisher(publisher, documentSL)
	ssAcquirer := publish.NewSingleStoreAcquirer(acquirer, documentSL)
	mlPublisher := publish.NewMultiLoadPublisher(slPublisher, config.Publish)
	msAcquirer := publish.NewMultiStoreAcquirer(ssAcquirer, config.Publish)
	shipper := ship.NewShipper(librarians, publisher, mlPublisher)
	receiver := ship.NewReceiver(librarians, selfReaderKeys, acquirer, msAcquirer, documentSL)

	mdEncDec := enc.NewMetadataEncrypterDecrypter()
	entryPacker := pack.NewEntryPacker(config.Print, mdEncDec, documentSL)
	entryUnpacker := pack.NewEntryUnpacker(config.Print, mdEncDec, documentSL)

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
		entryPacker:    entryPacker,
		entryUnpacker:  entryUnpacker,
		shipper:        shipper,
		receiver:       receiver,
		pageSL:         page.NewStorerLoader(documentSL),
		signer:         signer,
		logger:         logger,
		stop:           make(chan struct{}),
	}, nil
}

// TODO (drausin) Author methods
// - Share()

// Upload compresses, encrypts, and splits the content into pages and then stores them in the
// libri network. It returns the uploaded envelope for self-storage and its key.
func (a *Author) Upload(content io.Reader, mediaType string) (*api.Document, id.ID, error) {
	authorPub, readerPub, keys, err := a.envelopeKeys.sample()
	if err != nil {
		return nil, nil, err
	}

	a.logger.Debug("packing content",
		zap.String(LoggerAuthorPub, fmt.Sprintf("%065x", authorPub)),
	)
	entry, err := a.entryPacker.Pack(content, mediaType, keys, authorPub)
	if err != nil {
		return nil, nil, err
	}

	a.logger.Debug("shipping entry",
		zap.String(LoggerAuthorPub, fmt.Sprintf("%065x", authorPub)),
		zap.String(LoggerReaderPub, fmt.Sprintf("%065x", readerPub)),
	)
	envelope, envelopeKey, err := a.shipper.Ship(entry, authorPub, readerPub)
	if err != nil {
		return nil, nil, err
	}

	// TODO (drausin)
	// - delete pages from local storage

	entryKeyBytes := envelope.Contents.(*api.Document_Envelope).Envelope.EntryKey
	a.logger.Debug("successfully uploaded document",
		zap.String(LoggerEnvelopeKey, envelopeKey.String()),
		zap.String(LoggerEntryKey, id.FromBytes(entryKeyBytes).String()),
	)
	return envelope, envelopeKey, nil
}

// Download downloads, join, decrypts, and decompressed the content, writing it to a unified output
// content writer.
func (a *Author) Download(content io.Writer, envelopeKey id.ID) error {
	a.logger.Debug("receiving entry", zap.String(LoggerEnvelopeKey, envelopeKey.String()))
	entry, keys, err := a.receiver.Receive(envelopeKey)
	if err != nil {
		return err
	}
	entryKey, nPages, err := getEntryInfo(entry)
	if err != nil {
		return err
	}

	a.logger.Debug("unpacking content",
		zap.String(LoggerEntryKey, entryKey.String()),
		zap.Int(LoggerNPages, nPages),
	)
	if err := a.entryUnpacker.Unpack(content, entry, keys); err != nil {
		return err
	}

	// TODO (drausin)
	// - delete pages from local storage

	a.logger.Debug("successfully downloaded document",
		zap.String(LoggerEnvelopeKey, envelopeKey.String()),
		zap.String(LoggerEntryKey, entryKey.String()),
	)
	return nil
}

func getEntryInfo(entry *api.Document) (id.ID, int, error) {
	entryKey, err := api.GetKey(entry)
	if err != nil {
		return nil, 0, err
	}
	pageKeys, err := api.GetEntryPageKeys(entry)
	if err != nil {
		return nil, 0, err
	}
	if len(pageKeys) > 0 {
		return entryKey, len(pageKeys), nil
	}

	// zero extra pages implies a single page entry
	return entryKey, 1, nil
}