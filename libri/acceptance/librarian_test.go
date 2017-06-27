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
	"github.com/drausin/libri/libri/author/keychain"
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

type state struct {
	rng                 *rand.Rand
	client              *testClient
	seedConfigs         []*server.Config
	peerConfigs         []*server.Config
	seeds               []*server.Librarian
	peers               []*server.Librarian
	authors             []*lauthor.Author
	authorKeys          []keychain.Sampler
	logger              *zap.Logger
	putDocs             []*api.Document
	uploadedDocContents [][]byte
	uploadedDocEnvKeys  []id.ID
}

type params struct {
	nSeeds         int
	nPeers         int
	nAuthors       int
	logLevel       zapcore.Level
	nIntroductions int
	nPuts          int
	nUploads       int
}

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

	params := &params{
		nSeeds:         3,
		nPeers:         32,
		nAuthors:       3,
		logLevel:       zapcore.InfoLevel,
		nIntroductions: 16,
		nPuts:          16,
		nUploads:       16,
	}
	state := setUp(params)

	// healthcheck
	healthy, _ := state.authors[0].Healthcheck()
	assert.True(t, healthy)

	// ensure each peer can respond to an introduce request
	testIntroduce(t, params, state)

	// put a bunch of random data from random peers
	testPut(t, params, state)

	// get that same data from random peers
	testGet(t, params, state)

	// upload a bunch of random documents
	testUpload(t, params, state)
	//checkPublications(t, params, state)  // TODO (drausin) figure out why can be flakey

	// down the same ones
	testDownload(t, params, state)

	// share the uploaded docs with other author and download
	testShare(t, params, state)

	tearDown(state)

	awaitNewConnLogOutput()
}

func testIntroduce(t *testing.T, params *params, state *state) {
	nPeers := len(state.peers)
	q := lclient.NewIntroduceQuerier()

	// introduce oneself to a number of peers and ensure that each returns the requisite
	// number of new peers
	for c := 0; c < params.nIntroductions; c++ {

		// issue Introduce query to random peer
		i := state.rng.Int31n(int32(nPeers))
		conn := api.NewConnector(state.peerConfigs[i].PublicAddr)
		rq := lclient.NewIntroduceRequest(state.client.selfID, state.client.selfAPI, 8)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			search.DefaultQueryTimeout)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Introduce request",
			zap.String("to_peer", conn.Address().String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, int(rq.NumPeers), len(rp.Peers))
		state.client.logger.Debug("received Introduce response",
			zap.String("from_peer", conn.Address().String()),
			zap.Int("num_peers", len(rp.Peers)),
		)
	}
}

func testPut(t *testing.T, params *params, state *state) {
	q := lclient.NewPutQuerier()
	putDocs := make([]*api.Document, params.nPuts)

	// create a bunch of random putDocs to put
	for c := 0; c < params.nPuts; c++ {

		// create random bytes of length [2, 256)
		value, key := api.NewTestDocument(state.rng)
		putDocs[c] = value
		rq := lclient.NewPutRequest(state.client.selfID, key, value)

		// issue Put query to random peer
		i := state.rng.Int31n(int32(params.nPeers))
		conn := api.NewConnector(state.peerConfigs[i].PublicAddr)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Put request",
			zap.String("to_peer", conn.Address().String()),
			zap.String("key", key.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, api.PutOperation_STORED, rp.Operation)
		assert.True(t, uint32(state.peerConfigs[i].Search.NClosestResponses) <= rp.NReplicas)
		state.client.logger.Debug("received Put response",
			zap.String("from_peer", conn.Address().String()),
			zap.String("operation", rp.Operation.String()),
			zap.Int("n_replicas", int(rp.NReplicas)),
		)
	}

	state.putDocs = putDocs
}

func testGet(t *testing.T, params *params, state *state) {
	q := lclient.NewGetQuerier()

	// create a bunch of random values to put
	for c := 0; c < len(state.putDocs); c++ {

		// create Get request for value
		value := state.putDocs[c]
		key, err := api.GetKey(value)
		assert.Nil(t, err)
		rq := lclient.NewGetRequest(state.client.selfID, key)

		// issue Get query to random peer
		i := state.rng.Int31n(int32(params.nPeers))
		conn := api.NewConnector(state.peerConfigs[i].PublicAddr)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Get request",
			zap.String("to_peer", conn.Address().String()),
			zap.String("key", key.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()
		state.client.logger.Debug("received Get response",
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

func testUpload(t *testing.T, params *params, state *state) {

	contents := make([][]byte, params.nUploads)
	envKeys := make([]id.ID, params.nUploads)
	maxContentSize := 12 * 1024 * 1024
	minContentSize := 32
	var err error
	for i := 0; i < params.nUploads; i++ {
		nContentBytes := minContentSize +
			int(state.rng.Int31n(int32(maxContentSize-minContentSize)))
		contents[i] = common.NewCompressableBytes(state.rng, nContentBytes).Bytes()
		mediaType := "application/x-pdf"
		if state.rng.Int()%2 == 0 {
			mediaType = "application/x-gzip"
		}

		// upload the contents
		_, envKeys[i], err = state.authors[0].Upload(bytes.NewReader(contents[i]), mediaType)
		assert.Nil(t, err)
	}
	state.uploadedDocContents = contents
	state.uploadedDocEnvKeys = envKeys
}

func testDownload(t *testing.T, _ *params, state *state) {
	for i, envKey := range state.uploadedDocEnvKeys {
		downloaded := new(bytes.Buffer)
		err := state.authors[0].Download(downloaded, envKey)
		assert.Nil(t, err)
		assert.Equal(t, len(state.uploadedDocContents[i]), downloaded.Len())
		assert.Equal(t, state.uploadedDocContents[i], downloaded.Bytes())
	}
}

func checkPublications(t *testing.T, params *params, state *state) {

	receiveWaitTime := 10 * time.Second
	state.logger.Info("waiting for librarians to receive publications",
		zap.Float64("n_seconds", receiveWaitTime.Seconds()),
	)
	time.Sleep(receiveWaitTime)

	// check all peers have publications for all docs
	for i, p := range state.peers {
		info := fmt.Sprintf("peer %d", i)
		assert.Equal(t, params.nUploads, p.RecentPubs.Len(), info)
	}
}

func testShare(t *testing.T, _ *params, state *state) {
	from, to := state.authors[0], state.authors[1]
	toKeys := state.authorKeys[1]
	for i, origEnvKey := range state.uploadedDocEnvKeys {
		toKey, err := toKeys.Sample()
		assert.Nil(t, err)

		_, envKey, err := from.Share(origEnvKey, &toKey.Key().PublicKey)
		assert.Nil(t, err)

		downloaded := new(bytes.Buffer)
		err = to.Download(downloaded, envKey)
		assert.Nil(t, err)
		assert.Equal(t, len(state.uploadedDocContents[i]), downloaded.Len())
		assert.Equal(t, state.uploadedDocContents[i], downloaded.Bytes())
	}
}

// testClient has enough info to make requests to other peers
type testClient struct {
	selfID  ecid.ID
	selfAPI *api.PeerAddress
	signer  lclient.Signer
	logger  *zap.Logger
}

func setUp(params *params) *state {
	maxBucketPeers := uint(8)
	dataDir, err := ioutil.TempDir("", "test-data-dir")
	if err != nil {
		panic(err)
	}
	seedConfigs, peerConfigs, bootstrapAddrs := newLibrarianConfigs(
		dataDir,
		params.nSeeds,
		params.nPeers,
		maxBucketPeers,
		params.logLevel,
	)
	authorConfigs := newAuthorConfigs(dataDir, params.nAuthors, bootstrapAddrs, params.logLevel)
	seeds := make([]*server.Librarian, params.nSeeds)
	peers := make([]*server.Librarian, params.nPeers)
	logger := clogging.NewDevLogger(params.logLevel)
	seedsUp := make(chan *server.Librarian, 1)

	// create & start seeds
	for c := 0; c < params.nSeeds; c++ {
		logger.Info("starting seed",
			zap.String("seed_name", seedConfigs[c].PublicName),
			zap.String("seed_address", seedConfigs[c].PublicAddr.String()),
		)
		go func() { server.Start(logger, seedConfigs[c], seedsUp) }()
		seeds[c] = <-seedsUp // wait for seed to come up
	}

	// create & start other peers
	nShards := 4 // should be factor of nPeers
	if params.nPeers%nShards != 0 {
		nShards = 1
	}
	nPeersPerShard := params.nPeers / nShards
	var wg sync.WaitGroup
	for s := 0; s < nShards; s++ {
		wg.Add(1)
		go startLibrariansShard(&wg, s, nPeersPerShard, peers, peerConfigs, logger)
	}
	wg.Wait()

	subscriptionWaitTime := 5 * time.Second
	logger.Info("waiting for librarians to begin subscriptions",
		zap.Float64("n_seconds", subscriptionWaitTime.Seconds()),
	)
	time.Sleep(subscriptionWaitTime)

	// create client that will issue requests to network
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	publicAddr := peer.NewTestPublicAddr(params.nSeeds + params.nPeers + 1)
	selfPeer := peer.New(selfID.ID(), "test client", api.NewConnector(publicAddr))
	signer := lclient.NewSigner(selfID.Key())
	client := &testClient{
		selfID:  selfID,
		selfAPI: selfPeer.ToAPI(),
		signer:  signer,
		logger:  logger,
	}

	// create authors
	authors := make([]*lauthor.Author, len(authorConfigs))
	authorKeys := make([]keychain.Sampler, len(authorConfigs))
	for i, authorConfig := range authorConfigs {

		// create keychains
		err := lauthor.CreateKeychains(logger, authorConfig.KeychainDir, authorKeychainAuth,
			veryLightScryptN, veryLightScryptP)
		if err != nil {
			panic(err)
		}

		// load keychains
		authorKCs, selfReaderKCs, err := lauthor.LoadKeychains(authorConfig.KeychainDir,
			authorKeychainAuth)
		if err != nil {
			panic(err)
		}
		authorKeys[i] = authorKCs

		// create author
		authors[i], err = lauthor.NewAuthor(authorConfig, authorKCs, selfReaderKCs, logger)
		if err != nil {
			panic(err)
		}
	}

	return &state{
		rng:         rng,
		client:      client,
		seedConfigs: seedConfigs,
		peerConfigs: peerConfigs,
		seeds:       seeds,
		peers:       peers,
		authors:     authors,
		authorKeys:  authorKeys,
		logger:      logger,
	}
}

func startLibrariansShard(
	wg *sync.WaitGroup,
	shardIdx int,
	nPeersPerShard int,
	peers []*server.Librarian,
	peerConfigs []*server.Config,
	logger *zap.Logger,
) {
	defer wg.Done()
	shardPeersUp := make(chan *server.Librarian, 1)
	errs := make(chan error, 1)
	for c := 0; c < nPeersPerShard; c++ {
		i := shardIdx*nPeersPerShard + c
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
		case err := <-errs:
			panic(err)
		case peers[i] = <-shardPeersUp:
			// continue
		}
	}
}

func tearDown(state *state) {
	// disconnect from librarians and remove data dir
	for _, author := range state.authors {
		author.CloseAndRemove()
	}

	// gracefully shut down peers and seeds
	for _, p1 := range state.peers {
		go func(p2 *server.Librarian) {
			// explicitly end subscriptions first and then sleep so that later librarians
			// don't crash b/c of flurry of ended subscriptions from earlier librarians
			p2.EndSubscriptions()
			time.Sleep(3 * time.Second)
			p2.Close()
		}(p1)
	}
	for _, s := range state.seeds {
		s.Close()
	}

	// remove data dir shared by all
	os.RemoveAll(state.seedConfigs[0].DataDir)
}

func newLibrarianConfigs(dataDir string, nSeeds, nPeers int, maxBucketPeers uint,
	logLevel zapcore.Level) ([]*server.Config, []*server.Config, []*net.TCPAddr) {
	seedStartPort, peerStartPort := 12000, 13000

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

	return seedConfigs, peerConfigs, bootstrapAddrs
}

func newAuthorConfigs(dataDir string, nAuthors int, bootstrapAddrs []*net.TCPAddr,
	logLevel zapcore.Level) []*lauthor.Config {
	authorConfigs := make([]*lauthor.Config, nAuthors)
	for c := 0; c < nAuthors; c++ {
		authorDataDir := filepath.Join(dataDir, fmt.Sprintf("author-%d", c))
		authorConfigs[c] = lauthor.NewDefaultConfig().
			WithLibrarianAddrs(bootstrapAddrs).
			WithDataDir(authorDataDir).
			WithDefaultDBDir().
			WithDefaultKeychainDir().
			WithLogLevel(logLevel)
	}
	return authorConfigs
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
