package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"sync"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/parse"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestStart_ok(t *testing.T) {
	// start a single librarian server
	config := newTestConfig()
	config.WithProfile(true).WithReportMetrics(true)
	config.WithLogLevel(zapcore.DebugLevel)

	var err error
	up := make(chan *Librarian, 1)
	wg1 := new(sync.WaitGroup)
	wg1.Add(1)
	go func(wg2 *sync.WaitGroup) {
		defer wg2.Done()
		err = Start(clogging.NewDevInfoLogger(), config, up)
		assert.Nil(t, err)
	}(wg1)

	// get the librarian once it's up
	librarian := <-up

	// set up clients
	conn, err := grpc.Dial(fmt.Sprintf(":%d", config.LocalPort), grpc.WithInsecure())
	assert.NotNil(t, conn)
	assert.Nil(t, err)
	clientHealth := healthpb.NewHealthClient(conn)

	// confirm ok health check
	ctx1, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	rp, err := clientHealth.Check(ctx1, &healthpb.HealthCheckRequest{})
	cancel()
	assert.Nil(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, rp.Status)

	// confirm ok metrics
	metricsAddr := fmt.Sprintf("http://localhost:%d/metrics", config.LocalMetricsPort)
	resp, err := http.Get(metricsAddr)
	assert.Nil(t, err)
	assert.Equal(t, "200 OK", resp.Status)

	// confirm ok debug pprof info
	profilerAddr := fmt.Sprintf("http://localhost:%d/debug/pprof", config.LocalProfilerPort)
	resp, err = http.Get(profilerAddr)
	assert.Nil(t, err)
	assert.Equal(t, "200 OK", resp.Status)

	assert.Nil(t, librarian.CloseAndRemove())
	wg1.Wait()
}

func TestStart_newLibrarianErr(t *testing.T) {
	config := &Config{
		DataDir: "some/nonexistant/path",
	}

	// check that NewLibrarian error bubbles up
	assert.NotNil(t, Start(zap.NewNop(), config, make(chan *Librarian, 1)))
}

func TestStart_bootstrapPeersErr(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-start")
	defer func() { err = os.RemoveAll(dataDir) }()
	assert.Nil(t, err)
	config := NewDefaultConfig()
	config.WithDataDir(dataDir).WithDefaultDBDir()
	config.WithBootstrapAddrs(make([]*net.TCPAddr, 0))
	config.WithReportMetrics(false)

	// configure bootstrap peer to be non-existent peer
	publicAddr, err := parse.Addr(DefaultIP, DefaultPort-1)
	assert.Nil(t, err)
	config.BootstrapAddrs = append(config.BootstrapAddrs, publicAddr)

	// check that bootstrap error bubbles up
	assert.NotNil(t, Start(zap.NewNop(), config, make(chan *Librarian, 1)))
}

func TestLibrarian_bootstrapPeers_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nSeeds, nPeers := 3, 8

	// define the seeds
	seeds := make([]*net.TCPAddr, nSeeds)
	for i := 0; i < nSeeds; i++ {
		seeds[i] = peer.NewTestPublicAddr(i)
	}

	// define out fixed introduction result
	fixedResult := introduce.NewInitialResult()
	for _, p := range peer.NewTestPeers(rng, nPeers) {
		fixedResult.Responded[p.ID().String()] = p
	}

	p, d := &fixedPreferer{}, &fixedDoctor{}
	rt := routing.NewEmpty(id.NewPseudoRandom(rng), p, d, routing.NewDefaultParameters())
	l := &Librarian{
		config: NewDefaultConfig(),
		introducer: &fixedIntroducer{
			result: fixedResult,
		},
		rt:     rt,
		logger: zap.NewNop(),
	}

	err := l.bootstrapPeers(seeds)
	assert.Nil(t, err)

	// make sure all the responded peers are in the routing table
	for _, p := range fixedResult.Responded {
		q, exists := l.rt.Get(p.ID())
		assert.True(t, exists)
		assert.NotNil(t, q)
		assert.Equal(t, p, q)
	}
}

func TestLibrarian_bootstrapPeers_introduceErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nSeeds := 3

	// define the seeds
	seeds := make([]*net.TCPAddr, nSeeds)
	for i := 0; i < nSeeds; i++ {
		seeds[i] = peer.NewTestPublicAddr(i)
	}

	p, d := &fixedPreferer{}, &fixedDoctor{}
	rt := routing.NewEmpty(id.NewPseudoRandom(rng), p, d, routing.NewDefaultParameters())
	l := &Librarian{
		config: NewDefaultConfig(),
		selfID: ecid.NewPseudoRandom(rng),
		introducer: &fixedIntroducer{
			err: errors.New("some fatal introduce error"),
		},
		rt:     rt,
		logger: zap.NewNop(),
	}

	err := l.bootstrapPeers(seeds)

	// make sure error propagates
	assert.NotNil(t, err)
}

func TestLibrarian_bootstrapPeers_noResponsesErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nSeeds := 3

	// define the seeds
	seeds := make([]*net.TCPAddr, nSeeds)
	for i := 0; i < nSeeds; i++ {
		seeds[i] = peer.NewTestPublicAddr(i)
	}

	// fixed introduction result with no responses
	fixedResult := introduce.NewInitialResult()

	publicAddr, err := parse.Addr(DefaultIP, DefaultPort+1)
	assert.Nil(t, err)
	p, d := &fixedPreferer{}, &fixedDoctor{}
	rt := routing.NewEmpty(id.NewPseudoRandom(rng), p, d, routing.NewDefaultParameters())
	l := &Librarian{
		config: NewDefaultConfig().WithPublicAddr(publicAddr).WithLogLevel(zapcore.DebugLevel),
		selfID: ecid.NewPseudoRandom(rng),
		introducer: &fixedIntroducer{
			result: fixedResult,
		},
		rt:     rt,
		logger: zap.NewNop(),
	}

	err = l.bootstrapPeers(seeds)
	assert.NotNil(t, err)
}

type fixedIntroducer struct {
	result *introduce.Result
	err    error
}

func (fi *fixedIntroducer) Introduce(intro *introduce.Introduction, seeds []peer.Peer) error {
	intro.Result = fi.result
	return fi.err
}
