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
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/parse"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	lclient "github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/drausin/libri/libri/librarian/server/goodwill"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// nolint: megacheck
const (
	authorKeychainAuth = "acceptance test passphrase"

	veryLightScryptN = 2
	veryLightScryptP = 1

	artifactsDir     = "../../artifacts"
	benchmarksSubDir = "bench"
	benchmarksFile   = "librarian.bench"
)

// nolint: structcheck, megacheck
type state struct {
	rng                 *rand.Rand
	client              *testClient
	clients             client.Pool
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

// nolint: megacheck
type benchmarkObs struct {
	name    string
	procs   int
	results []testing.BenchmarkResult
}

// nolint: structcheck, megacheck
type params struct {
	nSeeds         int
	nPeers         int
	nAuthors       int
	logLevel       zapcore.Level
	nIntroductions int
	nPuts          int
	nUploads       int
	getTimeout     time.Duration
	putTimeout     time.Duration
}

// testClient has enough info to make requests to other peers
// nolint: megacheck
type testClient struct {
	selfID  ecid.ID
	selfAPI *api.PeerAddress
	signer  lclient.Signer
	rt      routing.Table
	logger  *zap.Logger
}

// nolint: megacheck
func setUp(params *params) *state { // nolint: deadcode
	maxBucketPeers := uint(8)
	dataDir, err := ioutil.TempDir("", "test-data-dir")
	errors.MaybePanic(err)
	seedConfigs, peerConfigs, peerAddrs := newLibrarianConfigs(
		dataDir,
		params.nSeeds,
		params.nPeers,
		maxBucketPeers,
		params.logLevel,
	)
	authorConfigs := newAuthorConfigs(dataDir, peerAddrs, params)
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
			err2 := server.Start(logger, seedConfigs[c], seedsUp)
			errors.MaybePanic(err2)
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
	selfPeer := peer.New(selfID.ID(), "test client", publicAddr)
	signer := lclient.NewSigner(selfID.Key())
	rec := goodwill.NewScalarRecorder()
	judge := goodwill.NewLatestNaiveJudge(rec)
	clientImpl := &testClient{
		selfID:  selfID,
		selfAPI: selfPeer.ToAPI(),
		rt:      routing.NewEmpty(selfID.ID(), judge, routing.NewDefaultParameters()),
		signer:  signer,
		logger:  logger,
	}

	// create authors
	authors := make([]*lauthor.Author, len(authorConfigs))
	authorKeys := make([]keychain.Sampler, len(authorConfigs))
	for i, authorConfig := range authorConfigs {

		// create keychains
		err = lauthor.CreateKeychains(logger, authorConfig.KeychainDir, authorKeychainAuth,
			veryLightScryptN, veryLightScryptP)
		errors.MaybePanic(err)

		// load keychains
		authorKCs, selfReaderKCs, err2 := lauthor.LoadKeychains(authorConfig.KeychainDir,
			authorKeychainAuth)
		errors.MaybePanic(err2)
		authorKeys[i] = authorKCs

		// create author
		authors[i], err2 = lauthor.NewAuthor(authorConfig, authorKCs, selfReaderKCs, logger)
		errors.MaybePanic(err2)
	}
	clients, err := client.NewDefaultLRUPool()
	errors.MaybePanic(err)

	return &state{
		rng:          rng,
		client:       clientImpl,
		clients:      clients,
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

// nolint: megacheck
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

// nolint: megacheck
func tearDown(state *state) { // nolint: deadcode
	// disconnect from librarians and remove data dir
	for _, author := range state.authors {
		err := author.CloseAndRemove()
		errors.MaybePanic(err)
	}

	// gracefully shut down peers and seeds
	all := append(state.peers, state.seeds...)
	for _, p1 := range all {
		go func(p2 *server.Librarian) {
			// explicitly end subscriptions first and then sleep so that later librarians
			// don't crash b/c of flurry of ended subscriptions from earlier librarians
			p2.StopAuxRoutines()
			time.Sleep(5 * time.Second)
			err := p2.Close()
			errors.MaybePanic(err)
		}(p1)
	}

	// remove data dir shared by all
	err := os.RemoveAll(state.seedConfigs[0].DataDir)
	errors.MaybePanic(err)
}

// nolint: megacheck
func writeBenchmarkResults(t *testing.T, benchmarks []*benchmarkObs) { // nolint: deadcode
	if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
		errors.MaybePanic(os.Mkdir(artifactsDir, 0700))
	}
	benchmarksDir := path.Join(artifactsDir, benchmarksSubDir)
	if _, err := os.Stat(benchmarksDir); os.IsNotExist(err) {
		errors.MaybePanic(os.Mkdir(benchmarksDir, 0700))
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
			_, err := fmt.Fprintf(f, "%-*s\t%s\n", maxNameLen, name, result.String())
			assert.Nil(t, err)
		}
	}
}

// nolint: megacheck
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

// nolint: megacheck
func newLibrarianConfigs(dataDir string, nSeeds, nPeers int, maxBucketPeers uint,
	logLevel zapcore.Level) ([]*server.Config, []*server.Config, []*net.TCPAddr) {
	seedStartPort, peerStartPort := 12000, 13000

	seedConfigs := make([]*server.Config, nSeeds)
	bootstrapAddrs := make([]*net.TCPAddr, nSeeds)
	for c := 0; c < nSeeds; c++ {
		localPort := seedStartPort + c
		seedConfigs[c] = newConfig(dataDir, localPort, maxBucketPeers, logLevel)
		bootstrapAddrs[c] = seedConfigs[c].PublicAddr
	}
	for c := 0; c < nSeeds; c++ {
		seedConfigs[c].WithBootstrapAddrs(bootstrapAddrs)
	}

	peerConfigs := make([]*server.Config, nPeers)
	peerAddrs := make([]*net.TCPAddr, nPeers)
	for c := 0; c < nPeers; c++ {
		localPort := peerStartPort + c
		peerConfigs[c] = newConfig(dataDir, localPort, maxBucketPeers, logLevel).
			WithBootstrapAddrs(bootstrapAddrs)
		peerAddrs[c] = peerConfigs[c].PublicAddr
	}

	return seedConfigs, peerConfigs, peerAddrs
}

// nolint: megacheck
func newAuthorConfigs(dataDir string, librarianAddrs []*net.TCPAddr, params *params,
) []*lauthor.Config {
	authorConfigs := make([]*lauthor.Config, params.nAuthors)
	for c := 0; c < params.nAuthors; c++ {
		authorDataDir := filepath.Join(dataDir, fmt.Sprintf("author-%d", c))
		publishParams := publish.NewDefaultParameters()

		// this is really long but adds robustness to our acceptance tests
		publishParams.GetTimeout = params.getTimeout
		publishParams.PutTimeout = params.putTimeout

		authorConfigs[c] = lauthor.NewDefaultConfig().
			WithLibrarianAddrs(librarianAddrs).
			WithDataDir(authorDataDir).
			WithDefaultDBDir().
			WithDefaultKeychainDir().
			WithLogLevel(params.logLevel).
			WithPublish(publishParams)
	}
	return authorConfigs
}

// nolint: megacheck
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

	localAddr, err := parse.Addr("localhost", port)
	errors.MaybePanic(err) // should never happen
	peerDataDir := filepath.Join(dataDir, server.NameFromAddr(localAddr))

	return server.NewDefaultConfig().
		WithLocalPort(port).
		WithReportMetrics(false).
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

// nolint: megacheck
func newTestDocument(rng *rand.Rand, entrySize int) (*api.Document, id.ID) { // nolint: deadcode
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
		Page: page,
	}
	doc := &api.Document{
		Contents: &api.Document_Entry{
			Entry: entry,
		},
	}
	key, err := api.GetKey(doc)
	errors.MaybePanic(err)
	return doc, key
}

// nolint: megacheck
func benchmarkName(name string, n int) string {
	if n != 1 {
		return fmt.Sprintf("Benchmark%s-%d", name, n)
	}
	return name
}

// nolint: megacheck
func getLibrarians(peerConfigs []*server.Config) (client.Balancer, error) { // nolint: deadcode
	rng := rand.New(rand.NewSource(0))
	librarianAddrs := make([]*net.TCPAddr, len(peerConfigs))
	for i, peerConfig := range peerConfigs {
		librarianAddrs[i] = peerConfig.PublicAddr
	}
	clients, err := lclient.NewDefaultLRUPool()
	if err != nil {
		return nil, err
	}
	return client.NewUniformBalancer(librarianAddrs, clients, rng)
}
