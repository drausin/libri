// +build acceptance

package acceptance

import (
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
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
)

// things to add later
// - random peer disconnects and additions
// - bad puts and gets
// - data persists after bouncing entire cluster

func TestLibrarianCluster(t *testing.T) {

	// handle grpc log noise
	restore := declareLogNoise(t,
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"transport: http2Server.HandleStreams failed to read frame",
		"context canceled; please retry",
		"grpc: the connection is closing; please retry",
	)
	defer restore()

	rng := rand.New(rand.NewSource(0))
	nSeeds, nPeers := 3, 32
	//clientLogLevel := zapcore.DebugLevel // handy for debugging test failures
	clientLogLevel := zapcore.InfoLevel
	client, seedConfigs, peerConfigs, seeds, peers := setUp(rng, nSeeds, nPeers, clientLogLevel)

	// ensure each peer can respond to an introduce request
	nIntroductions := 16
	testIntroduce(t, rng, client, peerConfigs, peers, nIntroductions)

	// put a bunch of random data from random peers
	nPuts := 16
	values := testPut(t, rng, client, peerConfigs, peers, nPuts)

	// get that same data from random peers
	testGet(t, rng, client, peerConfigs, peers, values)

	tearDown(seedConfigs, seeds, peers)

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
		conn := peer.NewConnector(peerConfigs[i].PublicAddr)
		rq := lclient.NewIntroduceRequest(client.selfID, client.selfAPI, 8)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(client.signer, rq,
			search.DefaultQueryTimeout)
		assert.Nil(t, err)
		client.logger.Debug("issuing Introduce request",
			zap.String("to_peer", conn.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()
		client.logger.Debug("received Introduce response",
			zap.String("from_peer", conn.String()),
			zap.Int("num_peers", len(rp.Peers)),
		)

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, int(rq.NumPeers), len(rp.Peers))
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
		conn := peer.NewConnector(peerConfigs[i].PublicAddr)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		client.logger.Debug("issuing Put request",
			zap.String("to_peer", conn.String()),
			zap.String("key", key.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()
		client.logger.Debug("received Put response",
			zap.String("from_peer", conn.String()),
			zap.String("operation", rp.Operation.String()),
			zap.Int("n_replicas", int(rp.NReplicas)),
		)

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, api.PutOperation_STORED, rp.Operation)
		assert.Equal(t, uint32(peerConfigs[i].Search.NClosestResponses), rp.NReplicas)
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
		conn := peer.NewConnector(peerConfigs[i].PublicAddr)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		client.logger.Debug("issuing Get request",
			zap.String("to_peer", conn.String()),
			zap.String("key", key.String()),
		)
		rp, err := q.Query(ctx, conn, rq)
		cancel()
		client.logger.Debug("received Get response",
			zap.String("from_peer", conn.String()),
		)

		// check everything went fine
		assert.Nil(t, err)
		rpKey, err := api.GetKey(rp.Value)
		assert.Nil(t, err)
		assert.Equal(t, key, rpKey)
		assert.Equal(t, value, rp.Value)
	}
}

// testClient has enough info to make requests to other peers
type testClient struct {
	selfID  ecid.ID
	selfAPI *api.PeerAddress
	signer  client.Signer
	logger  *zap.Logger
}

func setUp(rng *rand.Rand, nSeeds, nPeers int, logLevel zapcore.Level) (*testClient,
	[]*server.Config, []*server.Config, []*server.Librarian, []*server.Librarian) {
	maxBucketPeers := uint(8)
	seedConfigs, peerConfigs := newConfigs(nSeeds, nPeers, maxBucketPeers)
	seeds, peers := make([]*server.Librarian, nSeeds), make([]*server.Librarian, nPeers)
	logger := server.NewDevInfoLogger()
	seedsUp := make(chan *server.Librarian, 1)

	// create & start seeds
	for c := 0; c < nSeeds; c++ {
		logger.Info("starting seed", zap.String("seed_name", seedConfigs[c].PublicName))
		go func() { server.Start(logger, seedConfigs[c], seedsUp) }()
		seeds[c] = <-seedsUp // wait for seed to come up
	}

	// create & start other peers
	nShards := 4 // should be factor of nPeers
	nPeersPerShard := nPeers / nShards
	var wg sync.WaitGroup
	for b := 0; b < nShards; b++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, s int) {
			defer wg.Done()
			shardPeersUp := make(chan *server.Librarian, 1)
			for c := 0; c < nPeersPerShard; c++ {
				i := s*nPeersPerShard + c
				go func(j int) {
					server.Start(logger, peerConfigs[j], shardPeersUp)
				}(i)
				peers[i] = <-shardPeersUp
			}
		}(&wg, b)
	}
	wg.Wait()

	// create client that will issue requests to network
	selfID := ecid.NewPseudoRandom(rng)
	publicAddr := peer.NewTestPublicAddr(nSeeds + nPeers + 1)
	selfPeer := peer.New(selfID.ID(), "test client", peer.NewConnector(publicAddr))
	signer := client.NewSigner(selfID.Key())
	clientImpl := &testClient{
		selfID:  selfID,
		selfAPI: selfPeer.ToAPI(),
		signer:  signer,
		logger:  server.NewDevLogger(logLevel),
	}

	return clientImpl, seedConfigs, peerConfigs, seeds, peers
}

func tearDown(seedConfigs []*server.Config, seeds []*server.Librarian, peers []*server.Librarian) {
	// gracefully shut down peers and seeds
	for _, p := range peers {
		p.Close()
	}
	for _, s := range seeds {
		s.Close()
	}

	// remove data dir shared by all
	os.RemoveAll(seedConfigs[0].DataDir)
}

func newConfigs(nSeeds, nPeers int, maxBucketPeers uint) ([]*server.Config, []*server.Config) {
	seedStartPort, peerStartPort := 12000, 13000
	dataDir, err := ioutil.TempDir("", "test-data-dir")
	if err != nil {
		panic(err)
	}

	seedConfigs := make([]*server.Config, nSeeds)
	bootstrapAddrs := make([]*net.TCPAddr, nSeeds)
	for c := 0; c < nSeeds; c++ {
		seedConfigs[c] = newConfig(dataDir, seedStartPort+c, maxBucketPeers)
		bootstrapAddrs[c] = seedConfigs[c].PublicAddr
	}
	for c := 0; c < nSeeds; c++ {
		seedConfigs[c].WithBootstrapAddrs(bootstrapAddrs)
	}

	peerConfigs := make([]*server.Config, nPeers)
	for c := 0; c < nPeers; c++ {
		peerConfigs[c] = newConfig(dataDir, peerStartPort+c, maxBucketPeers).
			WithBootstrapAddrs(bootstrapAddrs)
	}

	return seedConfigs, peerConfigs
}

func newConfig(dataDir string, port int, maxBucketPeers uint) *server.Config {
	rtParams := routing.NewDefaultParameters()
	rtParams.MaxBucketPeers = maxBucketPeers

	introParams := introduce.NewDefaultParameters()
	introParams.TargetNumIntroductions = 16

	searchParams := search.NewDefaultParameters()
	searchParams.NClosestResponses = 3

	return server.NewDefaultConfig().
		WithLocalAddr(server.ParseAddr("localhost", port)).
		WithDefaultPublicAddr().
		WithDefaultPublicName().
		WithDefaultLocalName().
		WithDataDir(dataDir).
		WithDefaultDBDir().
		WithRouting(rtParams).
		WithIntroduce(introParams).
		WithSearch(searchParams)
}
