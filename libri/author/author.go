package author

import (
	"crypto/ecdsa"
	"io"
	"math/rand"
	"time"

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
	"golang.org/x/net/context"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	healthcheckTimeout = 2 * time.Second
)

// Author is the main client of the libri network. It can upload, download, and share documents with
// other author clients.
type Author struct {
	// ClientID is ID of this author client
	ClientID ecid.ID

	// Config holds the configuration parameters of the server
	config *Config

	// collection of keys for encrypting Envelope documents; these can be used as either the
	// author or reader keys
	authorKeys keychain.GetterSampler

	// collection of reader keys used with sending Envelope documents to oneself; these are
	// never used as the author key
	selfReaderKeys keychain.Getter

	// union of authorKeys and selfReaderKeys
	allKeys keychain.Getter

	// samples a pair of author and selfReader keys for encrypting an entry
	envKeys envelopeKeySampler

	// key-value store DB used for all external storage
	db db.KVDB

	// SL for client data
	clientSL storage.StorerLoader

	// SLD for locally stored documents
	documentSLD storage.DocumentSLD

	// load balancer for librarian clients
	librarians client.Balancer

	// librarian address -> health check client for all librarians
	librarianHealths map[string]healthpb.HealthClient

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
func NewAuthor(
	config *Config,
	authorKeys keychain.GetterSampler,
	selfReaderKeys keychain.GetterSampler,
	logger *zap.Logger,
) (*Author, error) {

	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		logger.Error("unable to init RocksDB", zap.Error(err))
		return nil, err
	}
	clientSL := storage.NewClientSL(rdb)

	// documentSL behaves more like a cache (i.e., everything is cleaned up), so ok for it to be
	// complete in-memory
	mdb := db.NewMemoryDB()
	documentSL := storage.NewDocumentSLD(mdb)

	// get client ID and immediately save it so subsequent restarts have it
	clientID, err := loadOrCreateClientID(logger, clientSL)
	if err != nil {
		return nil, err
	}
	clientLogger := logger.With(zap.String(logClientIDShort, id.ShortHex(clientID.Bytes())))

	allKeys := keychain.NewUnion(authorKeys, selfReaderKeys)
	envKeys := &envelopeKeySamplerImpl{
		authorKeys:     authorKeys,
		selfReaderKeys: selfReaderKeys,
	}

	// use client ID for rng seed so search client queries librarians in different order
	rng := rand.New(rand.NewSource(clientID.Int().Int64()))
	librarians, err := client.NewUniformBalancer(config.LibrarianAddrs, rng)
	if err != nil {
		return nil, err
	}
	getters := client.NewUniformGetterBalancer(librarians)
	putters := client.NewUniformPutterBalancer(librarians)
	librarianHealths, err := getLibrarianHealthClients(config.LibrarianAddrs)
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
	shipper := ship.NewShipper(putters, publisher, mlPublisher)
	receiver := ship.NewReceiver(getters, allKeys, acquirer, msAcquirer, documentSL)

	mdEncDec := enc.NewMetadataEncrypterDecrypter()
	entryPacker := pack.NewEntryPacker(config.Print, mdEncDec, documentSL)
	entryUnpacker := pack.NewEntryUnpacker(config.Print, mdEncDec, documentSL)

	author := &Author{
		ClientID:         clientID,
		config:           config,
		authorKeys:       authorKeys,
		selfReaderKeys:   selfReaderKeys,
		allKeys:          allKeys,
		envKeys:          envKeys,
		db:               rdb,
		clientSL:         clientSL,
		documentSLD:      documentSL,
		librarians:       librarians,
		librarianHealths: librarianHealths,
		entryPacker:      entryPacker,
		entryUnpacker:    entryUnpacker,
		shipper:          shipper,
		receiver:         receiver,
		pageSL:           page.NewStorerLoader(documentSL),
		signer:           signer,
		logger:           clientLogger,
		stop:             make(chan struct{}),
	}

	// for now, this doesn't really do anything
	go func() { <-author.stop }()

	return author, nil
}

// Healthcheck executes and reports healthcheck status for all connected librarians.
func (a *Author) Healthcheck() (bool, map[string]healthpb.HealthCheckResponse_ServingStatus) {
	healthStatus := make(map[string]healthpb.HealthCheckResponse_ServingStatus)
	allHealthy := true
	for addrStr, healthClient := range a.librarianHealths {
		ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
		rp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
		cancel()
		if err != nil {
			healthStatus[addrStr] = healthpb.HealthCheckResponse_UNKNOWN
			allHealthy = false
			a.logger.Info("librarian peer is not reachable",
				zap.String("peer_address", addrStr),
			)
			continue
		}

		healthStatus[addrStr] = rp.Status
		if rp.Status == healthpb.HealthCheckResponse_SERVING {
			a.logger.Info("librarian peer is healthy",
				zap.String("peer_address", addrStr),
			)
			continue
		}

		allHealthy = false
		a.logger.Warn("librarian peer is not healthy",
			zap.String("peer_address", addrStr),
		)

	}
	return allHealthy, healthStatus
}

// Upload compresses, encrypts, and splits the content into pages and then stores them in the
// libri network. It returns the uploaded envelope for self-storage and its key.
func (a *Author) Upload(content io.Reader, mediaType string) (*api.Document, id.ID, error) {
	startTime := time.Now()
	a.logger.Debug("uploading document")

	authorPub, readerPub, kek, eek, err := a.envKeys.sample()
	if err != nil {
		return nil, nil, a.logAndReturnErr("error sampling keys", err)
	}

	a.logger.Debug("packing content", packingContentFields(authorPub)...)
	entry, metadata, err := a.entryPacker.Pack(content, mediaType, eek, authorPub)
	if err != nil {
		return nil, nil, a.logAndReturnErr("error packing content", err)
	}

	a.logger.Debug("shipping entry", shippingEntryFields(authorPub, readerPub)...)
	env, envKey, err := a.shipper.ShipEntry(entry, authorPub, readerPub, kek, eek)
	if err != nil {
		return nil, nil, a.logAndReturnErr("error shipping entry", err)
	}

	elapsedTime := time.Since(startTime)
	a.logger.Info("uploaded document", uploadedDocFields(envKey, env, metadata, elapsedTime)...)
	return env, envKey, nil
}

// Download downloads, join, decrypts, and decompressed the content, writing it to a unified output
// content writer.
func (a *Author) Download(content io.Writer, envKey id.ID) error {
	startTime := time.Now()
	a.logger.Debug("downloading document", downloadingDocFields(envKey)...)

	entry, keys, err := a.receiver.ReceiveEntry(envKey)
	if err != nil {
		return a.logAndReturnErr("error receiving entry", err)
	}
	entryKey, nPages, err := getEntryInfo(entry)
	if err != nil {
		return a.logAndReturnErr("error getting entry info", err)
	}

	a.logger.Debug("unpacking content", unpackingContentFields(entryKey, nPages)...)
	metadata, err := a.entryUnpacker.Unpack(content, entry, keys)
	if err != nil {
		return a.logAndReturnErr("error unpacking content", err)
	}

	elapsedTime := time.Since(startTime)
	a.logger.Info("downloaded document",
		downloadedDocFields(envKey, entryKey, metadata, elapsedTime)...,
	)
	return nil
}

// Share creates and uploads a new envelope with the given reader public key. The new envelope
// has the same entry and entry encryption key as that of envelopeKey.
func (a *Author) Share(envKey id.ID, readerPub *ecdsa.PublicKey) (*api.Document, id.ID, error) {
	a.logger.Debug("sharing document", sharingDocFields(envKey, readerPub)...)
	env, err := a.receiver.ReceiveEnvelope(envKey)
	if err != nil {
		return nil, nil, a.logAndReturnErr("error receiving envelope", err)
	}
	return a.ShareEnvelope(env, readerPub)
}

// ShareEnvelope creates and uploads a new envelope with the given reader public key. The new
// envelope has the same entry and entry encryption key as the envelope passed in.
func (a *Author) ShareEnvelope(env *api.Envelope, readerPub *ecdsa.PublicKey) (
	*api.Document, id.ID, error) {
	eek, err := a.receiver.GetEEK(env)
	if err != nil {
		return nil, nil, a.logAndReturnErr("error getting EEK", err)
	}
	authorKey, err := a.authorKeys.Sample()
	if err != nil {
		return nil, nil, a.logAndReturnErr("error sampling author keys", err)
	}
	kek, err := enc.NewKEK(authorKey.Key(), readerPub)
	if err != nil {
		return nil, nil, a.logAndReturnErr("error creating new KEK", err)
	}
	entryKey := id.FromBytes(env.EntryKey)
	authKeyBs, readKeyBs := authorKey.PublicKeyBytes(), ecid.ToPublicKeyBytes(readerPub)
	sharedEnv, sharedEnvKey, err := a.shipper.ShipEnvelope(entryKey, authKeyBs, readKeyBs, kek, eek)
	if err != nil {
		return nil, nil, a.logAndReturnErr("error shipping envelope", err)
	}

	a.logger.Info("successfully shared document",
		sharedDocFields(sharedEnvKey, entryKey, authKeyBs, readKeyBs)...,
	)
	return sharedEnv, sharedEnvKey, nil
}

func (a *Author) logAndReturnErr(msg string, err error) error {
	a.logger.Error(msg, zap.Error(err))
	return err
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
