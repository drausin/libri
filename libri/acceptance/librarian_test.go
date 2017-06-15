// +build acceptance

package acceptance

import (
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"

	"bytes"
	"fmt"
	"time"

	lauthor "github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	lclient "github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"path/filepath"
	"github.com/drausin/libri/libri/common/subscribe"
)

// things to add later
// - random peer disconnects and additions
// - bad puts and gets
// - data persists after bouncing entire cluster

var (
	authorKeychainAuth = "acceptance test passphrase"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

func TestLibrarianCluster(t *testing.T) {

	// handle grpc log noise
	restore := declareLogNoise(t,
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"addrConn.resetTransport failed to create client transport",
		"transport: http2Server.HandleStreams failed to read frame",
		"transport: http2Server.HandleStreams failed to receive the preface from client: "+
			"EOF",
		"context canceled; please retry",
		"grpc: the connection is closing; please retry",
		"http2Client.notifyError got notified that the client transport was broken read",
		"http2Client.notifyError got notified that the client transport was broken EOF",
		"http2Client.notifyError got notified that the client transport was broken "+
			"write tcp",
	)
	defer restore()

	rng := rand.New(rand.NewSource(0))
	nSeeds, nPeers := 3, 32
	//logLevel := zapcore.DebugLevel // handy for debugging test failures
	logLevel := zapcore.InfoLevel
	client, seedConfigs, peerConfigs, seeds, peers, author, logger :=
		setUp(rng, nSeeds, nPeers, logLevel)

	// healthcheck
	healthy, _ := author.Healthcheck()
	assert.True(t, healthy)

	// ensure each peer can respond to an introduce request
	nIntroductions := 16
	testIntroduce(t, rng, client, peerConfigs, peers, nIntroductions)

	// put a bunch of random data from random peers
	nPuts := 16
	values := testPut(t, rng, client, peerConfigs, peers, nPuts)

	// get that same data from random peers
	testGet(t, rng, client, peerConfigs, peers, values)

	// upload a bunch of random documents
	nDocs := 16
	contents, envelopeKeys := testUpload(t, rng, author, nDocs)
	checkPublications(t, nDocs, peers, logger)

	// down the same ones
	testDownload(t, author, contents, envelopeKeys)

	tearDown(seedConfigs, seeds, peers, author)

	awaitNewConnLogOutput()
}

func testIntroduce(t *testing.T, rng *rand.Rand, client *testClient, peerConfigs []*server.Config,
	peers []*server.Librarian, nIntroductions int) {
	nPeers := len(peers)
	q := lclient.NewIntroduceQuerier()

	// introduce oneself to a number of peers and ensure that each returns the requisite
	// number of new peers
	for c := 0; c < nIntroductions; c++ {

		// issue Introduce query to random peer
		i := rng.Int31n(int32(nPeers))
		conn := api.NewConnector(peerConfigs[i].PublicAddr)
		rq := lclient.NewIntroduceRequest(client.selfID, client.selfAPI, 8)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(client.signer, rq,
			search.DefaultQueryTimeout)
		assert.Nil(t, err)
		client.logger.Debug("issuing Introduce request",
			zap.String("to_peer", conn.Address().String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, int(rq.NumPeers), len(rp.Peers))
		client.logger.Debug("received Introduce response",
			zap.String("from_peer", conn.Address().String()),
			zap.Int("num_peers", len(rp.Peers)),
		)
	}
}

func testPut(t *testing.T, rng *rand.Rand, client *testClient, peerConfigs []*server.Config,
	peers []*server.Librarian, nPuts int) []*api.Document {
	nPeers := len(peers)
	q := lclient.NewPutQuerier()
	values := make([]*api.Document, nPuts)

	// create a bunch of random values to put
	for c := 0; c < nPuts; c++ {

		// create random bytes of length [2, 256)
		value, key := api.NewTestDocument(rng)
		values[c] = value
		rq := lclient.NewPutRequest(client.selfID, key, value)

		// issue Put query to random peer
		i := rng.Int31n(int32(nPeers))
		conn := api.NewConnector(peerConfigs[i].PublicAddr)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		client.logger.Debug("issuing Put request",
			zap.String("to_peer", conn.Address().String()),
			zap.String("key", key.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, api.PutOperation_STORED, rp.Operation)
		assert.True(t, uint32(peerConfigs[i].Search.NClosestResponses) <= rp.NReplicas)
		client.logger.Debug("received Put response",
			zap.String("from_peer", conn.Address().String()),
			zap.String("operation", rp.Operation.String()),
			zap.Int("n_replicas", int(rp.NReplicas)),
		)
	}

	return values
}

func testGet(t *testing.T, rng *rand.Rand, client *testClient, peerConfigs []*server.Config,
	peers []*server.Librarian, values []*api.Document) {
	nPeers := len(peers)
	q := lclient.NewGetQuerier()

	// create a bunch of random values to put
	for c := 0; c < len(values); c++ {

		// create Get request for value
		value := values[c]
		key, err := api.GetKey(value)
		assert.Nil(t, err)
		rq := lclient.NewGetRequest(client.selfID, key)

		// issue Get query to random peer
		i := rng.Int31n(int32(nPeers))
		conn := api.NewConnector(peerConfigs[i].PublicAddr)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		client.logger.Debug("issuing Get request",
			zap.String("to_peer", conn.Address().String()),
			zap.String("key", key.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()
		client.logger.Debug("received Get response",
			zap.String("from_peer", conn.Address().String()),
		)

		// check everything went fine
		assert.Nil(t, err)
		rpKey, err := api.GetKey(rp.Value)
		assert.Nil(t, err)
		assert.Equal(t, key, rpKey)
		assert.Equal(t, value, rp.Value)
	}
}

func testUpload(t *testing.T, rng *rand.Rand, author *lauthor.Author, nDocs int) (
	contents [][]byte, envelopeKeys []id.ID) {

	contents = make([][]byte, nDocs)
	envelopeKeys = make([]id.ID, nDocs)
	maxContentSize := 12 * 1024 * 1024
	minContentSize := 32
	var err error
	for i := 0; i < nDocs; i++ {
		nContentBytes := minContentSize +
			int(rng.Int31n(int32(maxContentSize-minContentSize)))
		contents[i] = common.NewCompressableBytes(rng, nContentBytes).Bytes()
		mediaType := "application/x-pdf"
		if rng.Int()%2 == 0 {
			mediaType = "application/x-gzip"
		}

		// upload the contents
		_, envelopeKeys[i], err = author.Upload(bytes.NewReader(contents[i]), mediaType)
		assert.Nil(t, err)
	}
	return contents, envelopeKeys
}

func testDownload(t *testing.T, author *lauthor.Author, contents [][]byte, envelopeKeys []id.ID) {
	for i, envelopeKey := range envelopeKeys {
		downloaded := new(bytes.Buffer)
		err := author.Download(downloaded, envelopeKey)
		assert.Nil(t, err)
		assert.Equal(t, len(contents[i]), downloaded.Len())
		assert.Equal(t, contents[i], downloaded.Bytes())
	}
}

func checkPublications(t *testing.T, nDocs int, peers []*server.Librarian, logger *zap.Logger) {

	receiveWaitTime := 10 * time.Second
	logger.Info("waiting for librarians to receive publications",
		zap.Float64("n_seconds", receiveWaitTime.Seconds()),
	)
	time.Sleep(receiveWaitTime)

	// check all peers have publications for all docs
	for i, p := range peers {
		info := fmt.Sprintf("peer %d", i)
		assert.True(t, p.RecentPubs.Len() >= nDocs - 2, info)  // TODO (drausin) avoid buffer
	}
}

// testClient has enough info to make requests to other peers
type testClient struct {
	selfID  ecid.ID
	selfAPI *api.PeerAddress
	signer  lclient.Signer
	logger  *zap.Logger
}

func setUp(rng *rand.Rand, nSeeds, nPeers int, logLevel zapcore.Level) (
	client *testClient,
	seedConfigs []*server.Config,
	peerConfigs []*server.Config,
	seeds []*server.Librarian,
	peers []*server.Librarian,
	author *lauthor.Author,
	logger *zap.Logger,
) {
	maxBucketPeers := uint(8)
	seedConfigs, peerConfigs, authorConfig := newConfigs(nSeeds, nPeers, maxBucketPeers,
		logLevel)
	authorConfig.WithLogLevel(logLevel)
	seeds, peers = make([]*server.Librarian, nSeeds), make([]*server.Librarian, nPeers)
	logger = clogging.NewDevLogger(logLevel)
	seedsUp := make(chan *server.Librarian, 1)

	// create & start seeds
	for c := 0; c < nSeeds; c++ {
		logger.Info("starting seed",
			zap.String("seed_name", seedConfigs[c].PublicName),
			zap.String("seed_address", seedConfigs[c].PublicAddr.String()),
		)
		go func() { server.Start(logger, seedConfigs[c], seedsUp) }()
		seeds[c] = <-seedsUp // wait for seed to come up
	}

	// create & start other peers
	nShards := 4 // should be factor of nPeers
	if nPeers % nShards != 0 {
		nShards = 1
	}
	nPeersPerShard := nPeers / nShards
	var wg sync.WaitGroup
	errs := make(chan error, nShards)
	for b := 0; b < nShards; b++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, s int) {
			defer wg.Done()
			shardPeersUp := make(chan *server.Librarian, 1)
			for c := 0; c < nPeersPerShard; c++ {
				i := s*nPeersPerShard + c
				logger.Info("starting peer",
					zap.String("peer_name", peerConfigs[i].PublicName),
					zap.String("peer_address",
						peerConfigs[i].PublicAddr.String()),
				)
				go func(j int) {
					if err := server.Start(logger, peerConfigs[j], shardPeersUp); err != nil {
						errs <- err
					}

				}(i)
				select {
				case err := <- errs:
					panic(err)
				case peers[i] = <-shardPeersUp:
					// continue
				}
			}
		}(&wg, b)
	}
	wg.Wait()

	subscriptionWaitTime := 5 * time.Second
	logger.Info("waiting for librarians to begin subscriptions",
		zap.Float64("n_seconds", subscriptionWaitTime.Seconds()),
	)
	time.Sleep(subscriptionWaitTime)

	// create client that will issue requests to network
	selfID := ecid.NewPseudoRandom(rng)
	publicAddr := peer.NewTestPublicAddr(nSeeds + nPeers + 1)
	selfPeer := peer.New(selfID.ID(), "test client", api.NewConnector(publicAddr))
	signer := lclient.NewSigner(selfID.Key())
	client = &testClient{
		selfID:  selfID,
		selfAPI: selfPeer.ToAPI(),
		signer:  signer,
		logger:  logger,
	}

	// create keychains for author
	err := lauthor.CreateKeychains(logger, authorConfig.KeychainDir, authorKeychainAuth,
		veryLightScryptN, veryLightScryptP)
	if err != nil {
		panic(err)
	}

	authorKeys, selfReaderKeys, err := lauthor.LoadKeychains(authorConfig.KeychainDir,
		authorKeychainAuth)
	if err != nil {
		panic(err)
	}

	// create author
	author, err = lauthor.NewAuthor(authorConfig, authorKeys, selfReaderKeys, logger)
	if err != nil {
		panic(err)
	}

	return client, seedConfigs, peerConfigs, seeds, peers, author, logger
}

func tearDown(
	seedConfigs []*server.Config,
	seeds []*server.Librarian,
	peers []*server.Librarian,
	author *lauthor.Author,
) {
	// disconnect from librarians and remove data dir
	author.CloseAndRemove()

	// gracefully shut down peers and seeds
	for _, p1 := range peers {
		go func(p2 *server.Librarian) {
			p2.Close()
		}(p1)
	}
	for _, s := range seeds {
		s.Close()
	}

	// remove data dir shared by all
	os.RemoveAll(seedConfigs[0].DataDir)
}

func newConfigs(nSeeds, nPeers int, maxBucketPeers uint, logLevel zapcore.Level) (
	[]*server.Config, []*server.Config, *lauthor.Config) {
	seedStartPort, peerStartPort := 12000, 13000
	dataDir, err := ioutil.TempDir("", "test-data-dir")
	if err != nil {
		panic(err)
	}

	seedConfigs := make([]*server.Config, nSeeds)
	bootstrapAddrs := make([]*net.TCPAddr, nSeeds)
	for c := 0; c < nSeeds; c++ {
		seedConfigs[c] = newConfig(dataDir, seedStartPort+c, maxBucketPeers, logLevel)
		bootstrapAddrs[c] = seedConfigs[c].PublicAddr
	}
	for c := 0; c < nSeeds; c++ {
		seedConfigs[c].WithBootstrapAddrs(bootstrapAddrs)
	}

	peerConfigs := make([]*server.Config, nPeers)
	for c := 0; c < nPeers; c++ {
		peerConfigs[c] = newConfig(dataDir, peerStartPort+c, maxBucketPeers, logLevel).
			WithBootstrapAddrs(bootstrapAddrs)
	}

	authorConfig := lauthor.NewDefaultConfig().
		WithLibrarianAddrs(bootstrapAddrs).
		WithDataDir(dataDir).
		WithDefaultDBDir().
		WithDefaultKeychainDir().
		WithLogLevel(logLevel)
	authorConfig.Publish.PutTimeout = 10 * time.Second
	page.MinSize = 128                 // just for testing

	return seedConfigs, peerConfigs, authorConfig
}

func newConfig(
	dataDir string, port int, maxBucketPeers uint, logLevel zapcore.Level,
) *server.Config {

	rtParams := routing.NewDefaultParameters()
	rtParams.MaxBucketPeers = maxBucketPeers

	introParams := introduce.NewDefaultParameters()
	introParams.TargetNumIntroductions = 16

	searchParams := search.NewDefaultParameters()

	subscribeToParams := subscribe.NewDefaultToParameters()
	subscribeToParams.FPRate = 0.9

	localAddr, err := server.ParseAddr("localhost", port)
	if err != nil {
		// should never happen
		panic(err)
	}
	peerDataDir := filepath.Join(dataDir, server.NameFromAddr(localAddr))

	return server.NewDefaultConfig().
		WithLocalAddr(localAddr).
		WithDefaultPublicAddr().
		WithDefaultPublicName().
		WithDataDir(peerDataDir).
		WithDefaultDBDir().
		WithLogLevel(logLevel).
		WithRouting(rtParams).
		WithIntroduce(introParams).
		WithSearch(searchParams).
		WithSubscribeTo(subscribeToParams)
}
