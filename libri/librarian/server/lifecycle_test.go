package server

import (
	"io/ioutil"
	"math/rand"
	"net"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestStart_ok(t *testing.T) {
	// start a single librarian server
	config := newTestConfig()
	assert.True(t, config.isBootstrap())

	var err error
	up := make(chan *Librarian, 1)
	go func() {
		err = Start(clogging.NewDevInfoLogger(), config, up)
		assert.Nil(t, err)
	}()

	// get the librarian once it's up
	librarian := <-up

	// set up client
	conn, err := grpc.Dial(config.LocalAddr.String(), grpc.WithInsecure())
	assert.NotNil(t, conn)
	assert.Nil(t, err)
	client := api.NewLibrarianClient(conn)

	// confirm server is up and responds to ping
	rq1 := &api.PingRequest{}
	ctx1 := context.Background()
	rp1, err := client.Ping(ctx1, rq1)
	assert.Nil(t, err)
	assert.Equal(t, "pong", rp1.Message)

	assert.Nil(t, librarian.CloseAndRemove())
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
	assert.Nil(t, err)
	config := NewDefaultConfig()
	config.WithDataDir(dataDir).WithDefaultDBDir()
	config.WithBootstrapAddrs(make([]*net.TCPAddr, 0))

	// configure bootstrap peer to be non-existent peer
	config.BootstrapAddrs = append(config.BootstrapAddrs, ParseAddr(DefaultIP, DefaultPort+1))

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

	l := &Librarian{
		config: NewDefaultConfig(),
		introducer: &fixedIntroducer{
			result: fixedResult,
		},
		rt:     routing.NewEmpty(cid.NewPseudoRandom(rng), routing.NewDefaultParameters()),
		logger: clogging.NewDevInfoLogger(),
	}

	err := l.bootstrapPeers(seeds)
	assert.Nil(t, err)

	// make sure all the responded peers are in the routing table
	for _, p := range fixedResult.Responded {
		q := l.rt.Get(p.ID())
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

	l := &Librarian{
		config: NewDefaultConfig(),
		introducer: &fixedIntroducer{
			err: errorsNew("some fatal introduce error"),
		},
		rt:     routing.NewEmpty(cid.NewPseudoRandom(rng), routing.NewDefaultParameters()),
		logger: clogging.NewDevInfoLogger(),
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

	// define out fixed introduction result with no responses
	fixedResult := introduce.NewInitialResult()

	l := &Librarian{
		config: NewDefaultConfig().WithPublicAddr(ParseAddr(DefaultIP, DefaultPort+1)),
		introducer: &fixedIntroducer{
			result: fixedResult,
		},
		rt:     routing.NewEmpty(cid.NewPseudoRandom(rng), routing.NewDefaultParameters()),
		logger: clogging.NewDevInfoLogger(),
	}

	err := l.bootstrapPeers(seeds)
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
