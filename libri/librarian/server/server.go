package server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net/http"

	"time"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/replicate"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/drausin/libri/libri/librarian/server/verify"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/willf/bloom"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/status"
)

const (
	newPublicationsSlack = 16
)

var (
	errBadPeerIDSig           = errors.New("stated client peer ID does not match signature")
	errStoreUnexpectedResult  = errors.New("unexpected store result")
	errSearchUnexpectedResult = errors.New("unexpected search result")
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

	// replicates documents as needed
	replicator replicate.Replicator

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
	serverSL storage.StorerLoader

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

	// librarian client connection pool
	clients client.Pool

	// routing table of peers
	rt routing.Table

	// Prometheus counters for storage metrics
	storageMetrics *storageMetrics

	// recorder of query outcomes for each peer
	rec comm.QueryRecorder

	// determines whether requests are allowed
	allower comm.Allower

	// logger for this instance
	logger *zap.Logger

	// health server
	health *health.Server

	// metrics server
	metrics *http.Server

	// receives graceful stop signal
	stop chan struct{}

	// closed when server is stopped
	stopped chan struct{}
}

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
	selfID, err := loadOrCreatePeerID(logger, serverSL)
	if err != nil {
		return nil, err
	}
	selfLogger := logger.With(zap.String(logSelfIDShort, id.ShortHex(selfID.Bytes())))

	knower := comm.NewAlwaysKnower()
	doctor := comm.NewNaiveDoctor()

	// TODO (drausin) load recorder from storage instead of initializing empty
	windows := []time.Duration{comm.Second, comm.Day, comm.Week}
	recorder, getters := comm.NewWindowQueryRecorderGetters(knower, windows)
	if config.ReportMetrics {
		recorder = comm.NewPromScalarRecorder(selfID.ID(), recorder)
	}
	weekGetter := getters[7*24*time.Hour]
	prefer := comm.NewFindRpPreferer(weekGetter)
	allower := comm.NewDefaultAllower(knower, getters)

	rt, err := loadOrCreateRoutingTable(selfLogger, serverSL, prefer, doctor, selfID,
		config.Routing)
	if err != nil {
		return nil, err
	}
	clients, err := client.NewDefaultLRUPool()
	if err != nil {
		return nil, err
	}
	signer := client.NewSigner(selfID.Key())

	searcher := search.NewDefaultSearcher(signer, recorder, clients)
	storer := store.NewStorer(signer, recorder, searcher, client.NewStorerCreator(clients))
	introducer := introduce.NewDefaultIntroducer(signer, recorder, selfID.ID(), clients)
	verifier := verify.NewDefaultVerifier(signer, recorder, clients)

	newPubs := make(chan *subscribe.KeyedPub, newPublicationsSlack)
	recentPubs, err := subscribe.NewRecentPublications(config.SubscribeTo.RecentCacheSize)
	if err != nil {
		return nil, err
	}
	clientBalancer := routing.NewClientBalancer(rt, clients)
	subscribeTo := subscribe.NewTo(config.SubscribeTo, selfLogger, selfID, clientBalancer, signer,
		recentPubs, newPubs)

	metricsSM := http.NewServeMux()
	metricsSM.Handle("/metrics", promhttp.Handler())
	metrics := &http.Server{Addr: fmt.Sprintf(":%d", config.LocalMetricsPort), Handler: metricsSM}

	rng := rand.New(rand.NewSource(selfID.Int().Int64()))
	replicator := replicate.NewReplicator(
		selfID,
		rt,
		documentSL,
		verifier,
		storer,
		replicate.NewDefaultParameters(),
		verify.NewDefaultParameters(),
		config.Store,
		rng,
		selfLogger,
	)

	return &Librarian{
		selfID:         selfID,
		config:         config,
		apiSelf:        peer.FromAddress(selfID.ID(), config.PublicName, config.PublicAddr),
		introducer:     introducer,
		searcher:       searcher,
		replicator:     replicator,
		storer:         storer,
		subscribeFrom:  subscribe.NewFrom(config.SubscribeFrom, logger, newPubs),
		subscribeTo:    subscribeTo,
		RecentPubs:     recentPubs,
		rqv:            NewRequestVerifier(),
		db:             rdb,
		serverSL:       serverSL,
		documentSL:     documentSL,
		kc:             storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:            storage.NewHashKeyValueChecker(),
		fromer:         peer.NewFromer(),
		signer:         signer,
		clients:        clients,
		rt:             rt,
		storageMetrics: newStorageMetrics(),
		rec:            recorder,
		allower:        allower,
		logger:         selfLogger,
		health:         health.NewServer(),
		metrics:        metrics,
		stop:           make(chan struct{}),
		stopped:        make(chan struct{}),
	}, nil
}

// Introduce receives and gives identifying information about the peer in the network.
func (l *Librarian) Introduce(ctx context.Context, rq *api.IntroduceRequest) (
	*api.IntroduceResponse, error) {
	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received introduce request", introduceRequestFields(rq)...)
	endpoint := api.Introduce

	requesterID, err := l.checkRequest(ctx, rq, rq.Metadata)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, err)
	}
	if err := l.allower.Allow(requesterID, endpoint); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnNotAllowedErr(lg, err)
	}
	requester := l.fromer.FromAPI(rq.Self)
	if requester.ID().Cmp(requesterID) != 0 {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, errBadPeerIDSig)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

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
	lg.Info("introduced", introduceResponseFields(rp)...)
	return rp, nil
}

// Find returns either the value at a given target or the peers closest to it.
func (l *Librarian) Find(ctx context.Context, rq *api.FindRequest) (*api.FindResponse, error) {
	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received find request", findRequestFields(rq)...)
	endpoint := api.Find

	requesterID, err := l.checkRequestAndKey(ctx, rq, rq.Metadata, rq.Key)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, err)
	}
	if err = l.allower.Allow(requesterID, api.Find); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnNotAllowedErr(lg, err)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

	value, err := l.documentSL.Load(id.FromBytes(rq.Key))
	if err != nil {
		// something went wrong during load
		return nil, logReturnInternalErr(lg, "error loading document", err)
	}

	// we have the value, so return it
	if value != nil {
		rp := &api.FindResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
			Value:    value,
		}
		lg.Info("found value", findValueResponseFields(rq, rp)...)
		return rp, nil
	}

	// otherwise, return the peers closest to the key
	key := id.FromBytes(rq.Key)
	closest := l.rt.Find(key, uint(rq.NumPeers))
	rp := &api.FindResponse{
		Metadata: l.NewResponseMetadata(rq.Metadata),
		Peers:    peer.ToAPIs(closest),
	}
	lg.Info("found closest peers", findPeersResponseFields(rq, rp)...)
	return rp, nil
}

// Verify returns either the MAC of a value (if the peer has it) or the peers closest to it.
func (l *Librarian) Verify(
	ctx context.Context, rq *api.VerifyRequest,
) (*api.VerifyResponse, error) {

	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received verify request", verifyRequestFields(rq)...)
	endpoint := api.Verify

	requesterID, err := l.checkRequestAndKey(ctx, rq, rq.Metadata, rq.Key)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, err)
	}
	if err = l.allower.Allow(requesterID, endpoint); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnNotAllowedErr(lg, err)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

	mac, err := l.documentSL.Mac(id.FromBytes(rq.Key), rq.MacKey)
	if err != nil {
		// something went wrong during load
		return nil, logReturnInternalErr(lg, "error MACing document", err)
	}

	// we have the mac, so return it
	if mac != nil {
		rp := &api.VerifyResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
			Mac:      mac,
		}
		lg.Info("verified value", verifyMacResponseFields(rq, rp)...)
		return rp, nil
	}

	// otherwise, return the peers closest to the key
	key := id.FromBytes(rq.Key)
	closest := l.rt.Find(key, uint(rq.NumPeers))
	rp := &api.VerifyResponse{
		Metadata: l.NewResponseMetadata(rq.Metadata),
		Peers:    peer.ToAPIs(closest),
	}
	lg.Info("found closest peers", verifyPeersResponseFields(rq, rp)...)
	return rp, nil
}

// Store stores the value.
func (l *Librarian) Store(ctx context.Context, rq *api.StoreRequest) (
	*api.StoreResponse, error) {
	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received store request", storeRequestFields(rq)...)
	endpoint := api.Store

	requesterID, err := l.checkRequestAndKeyValue(ctx, rq, rq.Metadata, rq.Key, rq.Value)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, err)
	}
	if err := l.allower.Allow(requesterID, endpoint); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnNotAllowedErr(lg, err)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

	if err := l.documentSL.Store(id.FromBytes(rq.Key), rq.Value); err != nil {
		return nil, logReturnInternalErr(lg, "error storing document", err)
	}
	l.storageMetrics.Add(rq.Value)
	if err := l.subscribeTo.Send(api.GetPublication(rq.Key, rq.Value)); err != nil {
		return nil, logReturnInternalErr(lg, "error sending publication", err)
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
	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received get request", getRequestFields(rq)...)
	endpoint := api.Get

	requesterID, err := l.checkRequestAndKey(ctx, rq, rq.Metadata, rq.Key)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, err)
	}
	if err = l.allower.Allow(requesterID, endpoint); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnNotAllowedErr(lg, err)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

	key := id.FromBytes(rq.Key)
	s := search.NewSearch(l.selfID, key, l.config.Search)
	seeds := l.rt.Find(key, s.Params.NClosestResponses)
	if err = l.searcher.Search(s, seeds); err != nil {
		return nil, logReturnInternalErr(lg, "error searching", err)
	}

	// add found peers to routing table
	for _, p := range s.Result.Responded {
		l.rt.Push(p)
	}

	if s.FoundValue() {
		rp := &api.GetResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
			Value:    s.Result.Value,
		}
		lg.Info("got value", getResponseFields(rq, rp)...)
		return rp, nil
	}
	if s.FoundClosestPeers() {
		rp := &api.GetResponse{
			Metadata: l.NewResponseMetadata(rq.Metadata),
		}
		lg.Info("got closest peers", getResponseFields(rq, rp)...)
		return rp, nil
	}

	err, fs := s.Result.FatalErr, searchDetailFields(s)
	if s.Errored() {
		return nil, logReturnInternalErr(lg, "search errored", err, fs...)
	}
	if s.Exhausted() {
		return nil, logReturnUnavailErr(lg, "search exhausted closest peers", err, fs...)
	}

	return nil, logReturnInternalErr(lg, "search errored", errSearchUnexpectedResult, fs...)
}

// Put stores a given key and value. This endpoint handles the internals of finding the right
// peers to store the value in and then sending them store requests.
func (l *Librarian) Put(ctx context.Context, rq *api.PutRequest) (*api.PutResponse, error) {
	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received put request", putRequestFields(rq)...)
	endpoint := api.Put

	requesterID, err := l.checkRequestAndKeyValue(ctx, rq, rq.Metadata, rq.Key, rq.Value)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnInvalidRqErr(lg, err)
	}
	if err = l.allower.Allow(requesterID, endpoint); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return nil, logReturnNotAllowedErr(lg, err)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

	key := id.FromBytes(rq.Key)
	s := store.NewStore(
		l.selfID,
		key,
		rq.Value,
		l.config.Search,
		l.config.Store,
	)
	lg.Debug("beginning store queries", zap.String(logKey, id.Hex(rq.Key)))
	seeds := l.rt.Find(key, s.Search.Params.NClosestResponses)
	if err = l.storer.Store(s, seeds); err != nil {
		fs := storeDetailFields(s)
		return nil, logReturnInternalErr(lg, "error storing", err, fs...)
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
		lg.Info("put new value", putResponseFields(rq, rp)...)
		return rp, nil
	}
	if s.Exists() {
		rp := &api.PutResponse{
			Metadata:  l.NewResponseMetadata(rq.Metadata),
			Operation: api.PutOperation_LEFT_EXISTING,
			NReplicas: uint32(len(s.Result.Responded)),
		}
		lg.Info("put existing value", putResponseFields(rq, rp)...)
		return rp, nil
	}

	err, fs := s.Result.FatalErr, storeDetailFields(s)
	if s.Errored() {
		return nil, logReturnInternalErr(lg, "store errored", err, fs...)
	}
	if s.Exhausted() {
		return nil, logReturnUnavailErr(lg, "store exhausted", err, fs...)
	}
	return nil, logReturnInternalErr(lg, "store errored", errStoreUnexpectedResult)
}

// Subscribe begins a subscription to the peer's publication stream (from its own subscriptions to
// other peers).
func (l *Librarian) Subscribe(rq *api.SubscribeRequest, from api.Librarian_SubscribeServer) error {
	lg := l.logger.With(rqMetadataFields(rq.Metadata)...)
	lg.Debug("received subscribe request")
	endpoint := api.Subscribe

	requesterID, err := l.checkRequest(from.Context(), rq, rq.Metadata)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return logReturnInvalidRqErr(lg, err)
	}
	if err = l.allower.Allow(requesterID, endpoint); err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return logReturnNotAllowedErr(lg, err)
	}
	authorFilter, err := subscribe.FromAPI(rq.Subscription.AuthorPublicKeys)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return logReturnInvalidRqErr(lg, err)
	}
	readerFilter, err := subscribe.FromAPI(rq.Subscription.ReaderPublicKeys)
	if err != nil {
		l.record(requesterID, endpoint, comm.Request, comm.Error)
		return logReturnInvalidRqErr(lg, err)
	}
	l.record(requesterID, endpoint, comm.Request, comm.Success)

	pubs, done, err := l.subscribeFrom.New()
	if err != nil {
		lg.Info(err.Error(), zap.Error(err)) // Info b/c more of a business as usual response
		return status.Error(codes.ResourceExhausted, err.Error())
	}

	responseMetadata := l.NewResponseMetadata(rq.Metadata)
	for pub := range pubs {
		err = maybeSend(pub, authorFilter, readerFilter, from, responseMetadata, done)
		if err != nil {
			return logReturnUnavailErr(lg, "subscribe send error", err)
		}
	}

	// only close done if it's not already
	select {
	case <-done:
	default:
		close(done)
	}

	lg.Debug("finished with subscription")
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
