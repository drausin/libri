// +build acceptance

package acceptance

import (
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// things to add later
// - random peer disconnects and additions
// - bad puts and gets
// - data persists after bouncing entire cluster

func TestLibrarianCluster(t *testing.T) {
	restore := declareLogNoise(t,
		"grpc: addrConn.resetTransport failed to create client transport: connection error",
		"transport: http2Server.HandleStreams failed to read frame",
	)
	defer restore()
	rng := rand.New(rand.NewSource(0))
	nSeeds, nPeers := 3, 32
	seedConfigs, peerConfigs, seeds, peers := setUp(nSeeds, nPeers)

	// ensure each peer can respond to an introduce request
	nIntroductions := 16
	testIntroduce(t, rng, peerConfigs, peers, nSeeds, nIntroductions)

	// put a bunch of random data from random peers

	// get a bunch of random data from random peers

	tearDown(seedConfigs, seeds, peers)

}

func testIntroduce(t *testing.T, rng *rand.Rand, peerConfigs []*server.Config,
	peers []*server.Librarian, nSeeds, nIntroductions int) {
	nPeers := len(peers)
	selfID := ecid.NewPseudoRandom(rng)
	publicAddr := peer.NewTestPublicAddr(nSeeds + nPeers + 1)
	selfPeer := peer.New(selfID.ID(), "test client", peer.NewConnector(publicAddr))
	signer := signature.NewSigner(selfID.Key())

	q := client.NewIntroduceQuerier()

	// introduce oneself to a number of peers and ensure that each returns the requisite
	// number of new peers
	for c := 0; c < nIntroductions; c++ {
		i := rng.Int31n(int32(nPeers))
		conn := peer.NewConnector(peerConfigs[i].PublicAddr)
		rq := api.NewIntroduceRequest(selfID, selfPeer.ToAPI(), 8)
		ctx, cancel, err := client.NewSignedTimeoutContext(signer, rq,
			search.DefaultQueryTimeout)
		assert.Nil(t, err)
		rp, err := q.Query(ctx, conn, rq)
		cancel()
		assert.Nil(t, err)
		assert.Equal(t, int(rq.NumPeers), len(rp.Peers))
	}
}

func setUp(nSeeds, nPeers int) ([]*server.Config, []*server.Config, []*server.Librarian,
	[]*server.Librarian) {
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

	return seedConfigs, peerConfigs, seeds, peers
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

	return server.NewDefaultConfig().
		WithLocalAddr(server.ParseAddr("localhost", port)).
		WithDefaultPublicAddr().
		WithDefaultPublicName().
		WithDefaultLocalName().
		WithDataDir(dataDir).
		WithDefaultDBDir().
		WithRouting(rtParams).
		WithIntroduce(introParams)
}
