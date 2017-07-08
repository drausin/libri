package acceptance

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"path"

	lauthor "github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/api"
	lclient "github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	authorKeychainAuth = "acceptance test passphrase"

	veryLightScryptN = 2
	veryLightScryptP = 1

	benchmarksDir  = "../../bench"
	benchmarksFile = "librarian.bench"
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
	benchResults        []*benchmarkObs
}

type benchmarkObs struct {
	name    string
	procs   int
	results []testing.BenchmarkResult
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
	seedConfigs, peerConfigs, peerAddrs := newLibrarianConfigs(
		dataDir,
		params.nSeeds,
		params.nPeers,
		maxBucketPeers,
		params.logLevel,
	)
	authorConfigs := newAuthorConfigs(dataDir, params.nAuthors, peerAddrs, params.logLevel)
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
		go func() {
			err := server.Start(logger, seedConfigs[c], seedsUp)
			errors.MaybePanic(err)
		}()
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
		rng:          rng,
		client:       client,
		seedConfigs:  seedConfigs,
		peerConfigs:  peerConfigs,
		seeds:        seeds,
		peers:        peers,
		authors:      authors,
		authorKeys:   authorKeys,
		logger:       logger,
		benchResults: make([]*benchmarkObs, 0),
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
		err := author.CloseAndRemove()
		errors.MaybePanic(err)
	}

	// gracefully shut down peers and seeds
	for _, p1 := range state.peers {
		go func(p2 *server.Librarian) {
			// explicitly end subscriptions first and then sleep so that later librarians
			// don't crash b/c of flurry of ended subscriptions from earlier librarians
			p2.EndSubscriptions()
			time.Sleep(3 * time.Second)
			err := p2.Close()
			errors.MaybePanic(err)
		}(p1)
	}
	for _, s := range state.seeds {
		err := s.Close()
		errors.MaybePanic(err)
	}

	// remove data dir shared by all
	err := os.RemoveAll(state.seedConfigs[0].DataDir)
	errors.MaybePanic(err)
}

func writeBenchmarkResults(t *testing.T, benchmarks []*benchmarkObs) {
	if _, err := os.Stat(benchmarksDir); os.IsNotExist(err) {
		err = os.Mkdir(benchmarksDir, 0755)
		errors.MaybePanic(err)
	}
	f, err := os.Create(path.Join(benchmarksDir, benchmarksFile))
	defer func() {
		err = f.Close()
		errors.MaybePanic(err)
	}()
	assert.Nil(t, err)
	maxNameLen := len("Introduce")

	for _, benchmark := range benchmarks {
		name := benchmarkName(benchmark.name, benchmark.procs)
		for _, result := range averageSubsamples(benchmark.results, 4) {
			fmt.Fprintf(f, "%-*s\t%s\n", maxNameLen, name, result.String())
		}
	}
}

func averageSubsamples(
	original []testing.BenchmarkResult, subsampleSize int,
) []testing.BenchmarkResult {
	k := 0
	averagedLen := int(float32(len(original)/subsampleSize) + 0.99) // poor man's ceil()
	averaged := make([]testing.BenchmarkResult, averagedLen)

	for i, j := range rand.Perm(len(original)) {
		if i%subsampleSize == 0 {
			// create new average sample
			k = i / subsampleSize
			averaged[k] = testing.BenchmarkResult{}
		}

		// add random original sample to the average sample
		averaged[k].N += original[j].N
		averaged[k].T += original[j].T
		averaged[k].Bytes += original[j].Bytes
	}
	return averaged
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
	peerAddrs := make([]*net.TCPAddr, nPeers)
	for c := 0; c < nPeers; c++ {
		peerConfigs[c] = newConfig(dataDir, peerStartPort+c, maxBucketPeers, logLevel).
			WithBootstrapAddrs(bootstrapAddrs)
		peerAddrs[c] = peerConfigs[c].PublicAddr
	}

	return seedConfigs, peerConfigs, peerAddrs
}

func newAuthorConfigs(dataDir string, nAuthors int, librarianAddrs []*net.TCPAddr,
	logLevel zapcore.Level) []*lauthor.Config {
	authorConfigs := make([]*lauthor.Config, nAuthors)
	for c := 0; c < nAuthors; c++ {
		authorDataDir := filepath.Join(dataDir, fmt.Sprintf("author-%d", c))
		authorConfigs[c] = lauthor.NewDefaultConfig().
			WithLibrarianAddrs(librarianAddrs).
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

func newTestDocument(rng *rand.Rand, entrySize int) (*api.Document, id.ID) {
	page := &api.Page{
		AuthorPublicKey: api.RandBytes(rng, api.ECPubKeyLength),
		CiphertextMac:   api.RandBytes(rng, 32),
		Ciphertext:      api.RandBytes(rng, entrySize),
	}
	entry := &api.Entry{
		AuthorPublicKey:       page.AuthorPublicKey,
		CreatedTime:           1,
		MetadataCiphertext:    api.RandBytes(rng, 64),
		MetadataCiphertextMac: api.RandBytes(rng, api.HMAC256Length),
		Contents:              &api.Entry_Page{Page: page},
	}
	doc := &api.Document{
		Contents: &api.Document_Entry{
			Entry: entry,
		},
	}
	key, err := api.GetKey(doc)
	if err != nil {
		panic(err)
	}
	return doc, key
}

// benchmarkName returns full name of benchmark including procs suffix.
func benchmarkName(name string, n int) string {
	if n != 1 {
		return fmt.Sprintf("Benchmark%s-%d", name, n)
	}
	return name
}
