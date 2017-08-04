package server

import (
	"encoding/binary"
	"errors"
	"math/rand"

	"net/http"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/willf/bloom"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
	"google.golang.org/grpc/health"
)

// Librarian is the main service of a single peer in the peer to peer network.
type Librarian struct {
	// SelfID is the random 256-bit identification number of this node in the hash table
	selfID ecid.ID

	// Config holds the configuration parameters of the server
	config *Config

	// fixed API address
	apiSelf *api.PeerAddress

	// executes introductions to peers
	introducer introduce.Introducer

	// executes searches for peers and keys
	searcher search.Searcher

	// executes stores for key/value
	storer store.Storer

	// manages subscriptions from other peers
	subscribeFrom subscribe.From

	// manages subscriptions to other peers
	subscribeTo subscribe.To

	// RecentPubs is an LRU cache of recent publications librarian has received
	RecentPubs subscribe.RecentPublications

	// verifies requests from peers
	rqv RequestVerifier

	// key-value store DB used for all external storage
	db db.KVDB

	// SL for server data
	serverSL storage.NamespaceSL

	// SL for p2p stored documents
	documentSL storage.DocumentSL

	// ensures keys are valid
	kc storage.Checker

	// ensures keys and values are valid
	kvc storage.KeyValueChecker

	// creates new peers
	fromer peer.Fromer

	// signs requests
	signer client.Signer

	// routing table of peers
	rt routing.Table

	// logger for this instance
	logger *zap.Logger

	// health server
	health *health.Server

	// metrics server
	metrics *http.Server

	// receives graceful stop signal
	stop chan struct{}
}

const (
	newPublicationsSlack = 16
)

// NewLibrarian creates a new librarian instance.
func NewLibrarian(config *Config, logger *zap.Logger) (*Librarian, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		logger.Error("unable to init RocksDB", zap.Error(err))
		return nil, err
	}
	serverSL := storage.NewServerSL(rdb)
	documentSL := storage.NewDocumentSLD(rdb)

	// get peer ID and immediately save it so subsequent restarts have it
	peerID, err := loadOrCreatePeerID(logger, serverSL)
	if err != nil {
		return nil, err
	}
	selfLogger := logger.With(zap.String(logSelfIDShort, id.ShortHex(peerID.Bytes())))

	rt, err := loadOrCreateRoutingTable(selfLogger, serverSL, peerID, config.Routing)
	if err != nil {
		return nil, err
	}

	signer := client.NewSigner(peerID.Key())
	searcher := search.NewDefaultSearcher(signer)
	newPubs := make(chan *subscribe.KeyedPub, newPublicationsSlack)

	recentPubs, err := subscribe.NewRecentPublications(config.SubscribeTo.RecentCacheSize)
	if err != nil {
		return nil, err
	}
	clientBalancer := routing.NewClientBalancer(rt)
	subscribeTo := subscribe.NewTo(config.SubscribeTo, selfLogger, peerID, clientBalancer, signer,
		recentPubs, newPubs)

	metricsSM := http.NewServeMux()
	metricsSM.Handle("/metrics", promhttp.Handler())
	metrics := &http.Server{Addr: config.LocalMetricsAddr.String(), Handler: metricsSM}

	return &Librarian{
		selfID:        peerID,
		config:        config,
		apiSelf:       peer.FromAddress(peerID.ID(), config.PublicName, config.PublicAddr),
		introducer:    introduce.NewDefaultIntroducer(signer, peerID.ID()),
		searcher:      searcher,
		storer:        store.NewStorer(signer, searcher, client.NewStorerCreator()),
		subscribeFrom: subscribe.NewFrom(config.SubscribeFrom, logger, newPubs),
		subscribeTo:   subscribeTo,
		RecentPubs:    recentPubs,
		rqv:           NewRequestVerifier(),
		db:            rdb,
		serverSL:      serverSL,
		documentSL:    documentSL,
		kc:            storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:           storage.NewHashKeyValueChecker(),
		fromer:        peer.NewFromer(),
		signer:        signer,
		rt:            rt,
		logger:        selfLogger,
		health:        health.NewServer(),
		metrics:       metrics,
		stop:          make(chan struct{}),
	}, nil
}

var (
	errBadPeerIDSig           = errors.New("stated client peer ID does not match signature")
	errSearchErr              = errors.New("error encountered during search")
	errSearchExhausted        = errors.New("search exhausted closest peers")
	errSearchUnexpectedResult = errors.New("unexpected search result")
	errStoreErr               = errors.New("error storing")
	errStoreExhausted         = errors.New("store exhausted closest peers")
	errStoreUnexpectedResult  = errors.New("unexpected store result")
)

// Introduce receives and gives identifying information about the peer in the network.
func (l *Librarian) Introduce(ctx context.Context, rq *api.IntroduceRequest) (
	*api.IntroduceResponse, error) {
	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received introduce request", introduceRequestFields(rq)...)

	// check request
	requesterID, err := l.checkRequest(ctx, rq, rq.Metadata)
	if err != nil {
		return nil, logAndReturnErr(logger, "error checking request", err)
	}
	requester := l.fromer.FromAPI(rq.Self)
	if requester.ID().Cmp(requesterID) != 0 {
		return nil, logAndReturnErr(logger, "error matching peer ID to signature", errBadPeerIDSig)
	}
	l.record(requesterID, peer.Request, peer.Success)

	// add peer to routing table (if space)
	l.rt.Push(requester)

	// get random peers for client, using request ID as unique source of entropy for sample
	seed := int64(binary.BigEndian.Uint64(rq.Metadata.RequestId[:8]))
	peers := l.rt.Sample(uint(rq.NumPeers), rand.New(rand.NewSource(seed)))

	rp := &api.IntroduceResponse{
		Metadata: l.NewResponseMetadata(rq.Metadata),
		Self:     l.apiSelf,
		Peers:    peer.ToAPIs(peers),
	}
	logger.Info("introduced", introduceResponseFields(rp)...)
	return rp, nil
}

// Find returns either the value at a given target or the peers closest to it.
func (l *Librarian) Find(ctx context.Context, rq *api.FindRequest) (*api.FindResponse, error) {
	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received find request", findRequestFields(rq)...)

	requesterID, err := l.checkRequestAndKey(ctx, rq, rq.Metadata, rq.Key)
	if err != nil {
		return nil, logAndReturnErr(logger, "check request error", err)
	}
	l.record(requesterID, peer.Request, peer.Success)

	value, err := l.documentSL.Load(id.FromBytes(rq.Key))
	if err != nil {
		// something went wrong during load
		return nil, logAndReturnErr(logger, "error loading document", err)
	}

	// we have the value, so return it
	if value != nil {
		rp := &api.FindResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
			Value:    value,
		}
		logger.Info("found value", findValueResponseFields(rq, rp)...)
		return rp, nil
	}

	// otherwise, return the peers closest to the key
	key := id.FromBytes(rq.Key)
	closest := l.rt.Peak(key, uint(rq.NumPeers))
	rp := &api.FindResponse{
		Metadata: l.NewResponseMetadata(rq.Metadata),
		Peers:    peer.ToAPIs(closest),
	}
	logger.Info("found closest peers", findPeersResponseFields(rq, rp)...)
	return rp, nil
}

// Verify returns either the MAC of a value (if the peer has it) or the peers closest to it.
func (l *Librarian) Verify(ctx context.Context, rq *api.VerifyRequest) (
	*api.VerifyResponse, error) {

	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received verify request", verifyRequestFields(rq)...)

	requesterID, err := l.checkRequestAndKey(ctx, rq, rq.Metadata, rq.Key)
	if err != nil {
		return nil, logAndReturnErr(logger, "check request error", err)
	}
	l.record(requesterID, peer.Request, peer.Success)

	mac, err := l.documentSL.Mac(id.FromBytes(rq.Key), rq.MacKey)
	if err != nil {
		// something went wrong during load
		return nil, logAndReturnErr(logger, "error MACing document", err)
	}

	// we have the mac, so return it
	if mac != nil {
		rp := &api.VerifyResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
			Mac:      mac,
		}
		logger.Info("verified value", verifyMacResponseFields(rq, rp)...)
		return rp, nil
	}

	// otherwise, return the peers closest to the key
	key := id.FromBytes(rq.Key)
	closest := l.rt.Peak(key, uint(rq.NumPeers))
	rp := &api.VerifyResponse{
		Metadata: l.NewResponseMetadata(rq.Metadata),
		Peers:    peer.ToAPIs(closest),
	}
	logger.Info("found closest peers", verifyPeersResponseFields(rq, rp)...)
	return rp, nil
}

// Store stores the value.
func (l *Librarian) Store(ctx context.Context, rq *api.StoreRequest) (
	*api.StoreResponse, error) {
	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received store request", storeRequestFields(rq)...)

	requesterID, err := l.checkRequestAndKeyValue(ctx, rq, rq.Metadata, rq.Key, rq.Value)
	if err != nil {
		return nil, logAndReturnErr(logger, "error checking request", err)
	}
	l.record(requesterID, peer.Request, peer.Success)

	if err := l.documentSL.Store(id.FromBytes(rq.Key), rq.Value); err != nil {
		return nil, logAndReturnErr(logger, "error storing document", err)
	}
	if err := l.subscribeTo.Send(api.GetPublication(rq.Key, rq.Value)); err != nil {
		return nil, logAndReturnErr(logger, "error sending publication", err)
	}

	rp := &api.StoreResponse{
		Metadata: l.NewResponseMetadata(rq.Metadata),
	}
	l.logger.Debug("stored", storeResponseFields(rq, rp)...)
	return rp, nil
}

// Get returns the value for a given key, if it exists. This endpoint handles the internals of
// searching for the key.
func (l *Librarian) Get(ctx context.Context, rq *api.GetRequest) (*api.GetResponse, error) {
	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received get request", getRequestFields(rq)...)

	requesterID, err := l.checkRequestAndKey(ctx, rq, rq.Metadata, rq.Key)
	if err != nil {
		logger.Error("error checking request", zap.Error(err))
		return nil, err
	}
	l.record(requesterID, peer.Request, peer.Success)

	key := id.FromBytes(rq.Key)
	s := search.NewSearch(l.selfID, key, l.config.Search)
	seeds := l.rt.Peak(key, s.Params.NClosestResponses)
	if err = l.searcher.Search(s, seeds); err != nil {
		return nil, logAndReturnErr(logger, "error searching", err)
	}

	// add found peers to routing table
	for _, p := range s.Result.Closest.Peers() {
		l.rt.Push(p)
	}

	if s.FoundValue() {
		rp := &api.GetResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
			Value:    s.Result.Value,
		}
		logger.Info("got value", getResponseFields(rq, rp)...)
		return rp, nil
	}
	if s.FoundClosestPeers() {
		rp := &api.GetResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
		}
		logger.Info("got closest peers", getResponseFields(rq, rp)...)
		return rp, nil
	}
	if s.Errored() {
		return nil, logFieldsAndReturnErr(logger, errSearchErr, searchDetailFields(s))
	}
	if s.Exhausted() {
		return nil, logFieldsAndReturnErr(logger, errSearchExhausted, searchDetailFields(s))
	}

	return nil, logFieldsAndReturnErr(logger, errSearchUnexpectedResult, []zapcore.Field{})
}

// Put stores a given key and value. This endpoint handles the internals of finding the right
// peers to store the value in and then sending them store requests.
func (l *Librarian) Put(ctx context.Context, rq *api.PutRequest) (*api.PutResponse, error) {
	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received put request", putRequestFields(rq)...)

	requesterID, err := l.checkRequestAndKeyValue(ctx, rq, rq.Metadata, rq.Key, rq.Value)
	if err != nil {
		return nil, logAndReturnErr(logger, "error checking request", err)
	}
	l.record(requesterID, peer.Request, peer.Success)

	key := id.FromBytes(rq.Key)
	s := store.NewStore(
		l.selfID,
		key,
		rq.Value,
		l.config.Search,
		l.config.Store,
	)
	seeds := l.rt.Peak(key, s.Search.Params.NClosestResponses)
	if err = l.storer.Store(s, seeds); err != nil {
		return nil, logFieldsAndReturnErr(logger, errStoreErr, storeDetailFields(s))
	}
	for _, p := range s.Result.Responded {
		l.rt.Push(p)
	}
	if s.Stored() {
		rp := &api.PutResponse{
			Metadata:  l.NewResponseMetadata(rq.Metadata),
			Operation: api.PutOperation_STORED,
			NReplicas: uint32(len(s.Result.Responded)),
		}
		logger.Info("put new value", putResponseFields(rq, rp)...)
		return rp, nil
	}
	if s.Exists() {
		rp := &api.PutResponse{
			Metadata:  l.NewResponseMetadata(rq.Metadata),
			Operation: api.PutOperation_LEFT_EXISTING,
			NReplicas: uint32(len(s.Result.Responded)),
		}
		logger.Info("put existing value", putResponseFields(rq, rp)...)
		return rp, nil
	}
	if s.Errored() {
		return nil, logFieldsAndReturnErr(logger, errStoreErr, storeDetailFields(s))
	}
	if s.Exhausted() {
		return nil, logFieldsAndReturnErr(logger, errStoreExhausted, storeDetailFields(s))
	}
	return nil, logFieldsAndReturnErr(logger, errStoreUnexpectedResult, []zapcore.Field{})
}

// Subscribe begins a subscription to the peer's publication stream (from its own subscriptions to
// other peers).
func (l *Librarian) Subscribe(rq *api.SubscribeRequest, from api.Librarian_SubscribeServer) error {
	logger := l.logger.With(rqMetadataFields(rq.Metadata)...)
	logger.Debug("received subscribe request")
	if _, err := l.checkRequest(from.Context(), rq, rq.Metadata); err != nil {
		logger.Error("error checking request", zap.Error(err))
		return err
	}
	authorFilter, err := subscribe.FromAPI(rq.Subscription.AuthorPublicKeys)
	if err != nil {
		return logAndReturnErr(logger, "error decoding author filter", err)
	}
	readerFilter, err := subscribe.FromAPI(rq.Subscription.ReaderPublicKeys)
	if err != nil {
		return logAndReturnErr(logger, "error decoding reader filter", err)
	}
	pubs, done, err := l.subscribeFrom.New()
	if err != nil {
		logger.Info(err.Error(), zap.Error(err)) // Info b/c more of a business as usual response
		return err
	}

	responseMetadata := l.NewResponseMetadata(rq.Metadata)
	for pub := range pubs {
		err = maybeSend(pub, authorFilter, readerFilter, from, responseMetadata, done)
		if err != nil {
			return logAndReturnErr(logger, "subscribe send error", err)
		}
	}

	// only close done if it's not already
	select {
	case <-done:
	default:
		close(done)
	}

	logger.Debug("finished with subscription")
	return nil
}

func maybeSend(
	pub *subscribe.KeyedPub,
	authorFilter *bloom.BloomFilter,
	readerFilter *bloom.BloomFilter,
	from api.Librarian_SubscribeServer,
	responseMetadata *api.ResponseMetadata,
	done chan struct{},
) error {

	if !authorFilter.Test(pub.Value.AuthorPublicKey) {
		return nil
	}
	if !readerFilter.Test(pub.Value.ReaderPublicKey) {
		return nil
	}

	// if we get to here, we know that both author and reader keys are in the filters,
	// so we want to send the response
	rp := &api.SubscribeResponse{
		Metadata: responseMetadata,
		Key:      pub.Key.Bytes(),
		Value:    pub.Value,
	}
	if err := from.Send(rp); err != nil {
		close(done) // signal to l.subscribeFrom we're finished with this fanout
		return err
	}
	return nil
}
