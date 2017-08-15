// +build acceptance

package acceptance

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"
	"time"

	"crypto/rand"

	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	lclient "github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/drausin/libri/libri/librarian/server/verify"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	putPageSize = 1024 * 1024

	// for benchmark naming
	introduceName = "Introduce"
	putName       = "Put"
	getName       = "Get"
)

var grpcLogNoise = []string{
	"grpc: addrConn.resetTransport failed to create client transport: connection error",
	"addrConn.resetTransport failed to create client transport",
	"transport: http2Server.HandleStreams failed to read frame",
	"transport: http2Server.HandleStreams failed to receive the preface from client: EOF",
	"context canceled; please retry",
	"grpc: the connection is closing; please retry",
	"http2Client.notifyError got notified that the client transport was broken read",
	"http2Client.notifyError got notified that the client transport was broken EOF",
	"http2Client.notifyError got notified that the client transport was broken write tcp",
}

// things to add later
// - random peer disconnects and additions
// - bad puts and gets
// - data persists after bouncing entire cluster

func TestLibrarianCluster(t *testing.T) {

	// handle grpc log noise
	restore := declareLogNoise(t, grpcLogNoise...)
	defer restore()

	params := &params{
		nSeeds:         3,
		nPeers:         32,
		nAuthors:       3,
		logLevel:       zapcore.InfoLevel,
		nIntroductions: 32,
		nPuts:          128,
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

	//testReplicate(t, params, state)

	tearDown(state)

	writeBenchmarkResults(t, state.benchResults)

	awaitNewConnLogOutput()
}

func testIntroduce(t *testing.T, params *params, state *state) {
	nPeers := len(state.peers)
	ic := lclient.NewIntroducerCreator()
	benchResults := make([]testing.BenchmarkResult, params.nIntroductions)
	fromer := peer.NewFromer()

	// introduce oneself to a number of peers and ensure that each returns the requisite
	// number of new peers
	for c := 0; c < params.nIntroductions; c++ {
		start := time.Now()

		// issue Introduce query to random peer
		i := state.rng.Int31n(int32(nPeers))
		conn := peer.NewConnector(state.peerConfigs[i].PublicAddr)
		rq := lclient.NewIntroduceRequest(state.client.selfID, state.client.selfAPI, 8)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			search.DefaultQueryTimeout)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Introduce request",
			zap.String("to_peer", conn.Address().String()),
		)
		introducer, err := ic.Create(conn)
		assert.Nil(t, err)
		rp, err := introducer.Introduce(ctx, rq)
		cancel()

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, int(rq.NumPeers), len(rp.Peers))
		if rp != nil {
			state.client.logger.Debug("received Introduce response",
				zap.String("from_peer", conn.Address().String()),
				zap.Int("num_peers", len(rp.Peers)),
			)
		}

		benchResults[c] = testing.BenchmarkResult{
			N: 1,
			T: time.Now().Sub(start),
		}

		// add peers to self RT
		for _, p := range rp.Peers {
			state.client.rt.Push(fromer.FromAPI(p))
		}
	}

	state.benchResults = append(state.benchResults, &benchmarkObs{
		name:    introduceName,
		procs:   runtime.NumCPU(),
		results: benchResults,
	})
}

func testPut(t *testing.T, params *params, state *state) {
	putDocs := make([]*api.Document, params.nPuts)
	benchResults := make([]testing.BenchmarkResult, params.nPuts)
	librarians, err := getLibrarians(state.peerConfigs)
	assert.Nil(t, err)
	putters := client.NewUniformPutterBalancer(librarians)
	rlc := lclient.NewRetryPutter(putters, store.DefaultQueryTimeout)

	// create a bunch of random putDocs to put
	for c := 0; c < params.nPuts; c++ {
		start := time.Now()

		value, key := newTestDocument(state.rng, putPageSize)
		putDocs[c] = value
		rq := lclient.NewPutRequest(state.client.selfID, key, value)

		// issue Put query to random peer
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Put request", zap.String("key", key.String()))
		rp, err := rlc.Put(ctx, rq)
		cancel()
		benchResults[c] = testing.BenchmarkResult{
			N:     1,
			T:     time.Now().Sub(start),
			Bytes: int64(putPageSize),
		}

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, api.PutOperation_STORED, rp.Operation)
		if rp != nil {
			state.client.logger.Debug("received Put response",
				zap.String("operation", rp.Operation.String()),
				zap.Int("n_replicas", int(rp.NReplicas)),
			)
		}
	}

	state.benchResults = append(state.benchResults, &benchmarkObs{
		name:    putName,
		procs:   runtime.NumCPU(),
		results: benchResults,
	})
	state.putDocs = putDocs
}

func testGet(t *testing.T, params *params, state *state) {
	benchResults := make([]testing.BenchmarkResult, params.nPuts)
	librarians, err := getLibrarians(state.peerConfigs)
	assert.Nil(t, err)
	getters := client.NewUniformGetterBalancer(librarians)
	rlc := lclient.NewRetryGetter(getters, search.DefaultQueryTimeout)

	// create a bunch of random values to put
	for c := 0; c < len(state.putDocs); c++ {
		start := time.Now()

		// create Get request for value
		value := state.putDocs[c]
		key, err := api.GetKey(value)
		assert.Nil(t, err)
		rq := lclient.NewGetRequest(state.client.selfID, key)

		ctx, err := lclient.NewSignedContext(state.client.signer, rq)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Get request", zap.String("key", key.String()))
		rp, err := rlc.Get(ctx, rq)
		state.client.logger.Debug("received Get response", zap.String("key", key.String()))
		benchResults[c] = testing.BenchmarkResult{
			N:     1,
			T:     time.Now().Sub(start),
			Bytes: int64(putPageSize),
		}

		// check everything went fine
		assert.Nil(t, err)
		rpKey, err := api.GetKey(rp.Value)
		assert.Nil(t, err)
		assert.Equal(t, key, rpKey)
		assert.Equal(t, value, rp.Value)
	}

	state.benchResults = append(state.benchResults, &benchmarkObs{
		name:    getName,
		procs:   runtime.NumCPU(),
		results: benchResults,
	})
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

func testReplicate(t *testing.T, params *params, state *state) {

	// take n peers out of the network
	// verify each doc, seeing which are under-replicated
	// wait a bit
	// verify each doc again, confirming all fully-replicated
	// verify that NReplcated on librarians also increased
}

func countDocReplicas(t *testing.T, state *state) map[string]int {
	verifier := verify.NewDefaultVerifier(client.NewSigner(state.client.selfID.Key()))
	verifyParams := verify.NewDefaultParameters()
	verifyParams.NReplicas++         // since not accounting for self
	verifyParams.NClosestResponses++ // same

	nReplicas := make(map[string]int)
	for _, doc := range state.putDocs {
		key, err := api.GetKey(doc)
		assert.Nil(t, err)
		macKey := make([]byte, 32)
		_, err = rand.Read(macKey)
		assert.Nil(t, err)
		docBytes, err := proto.Marshal(doc)
		assert.Nil(t, err)

		seeds := state.client.rt.Peak(key, verifyParams.Concurrency)
		v := verify.NewVerify(state.client.selfID, key, docBytes, macKey, verifyParams)
		err = verifier.Verify(v, seeds)
		assert.Nil(t, err)
		nReplicas[key.String()] = len(v.Result.Replicas)
	}
	return nReplicas
}
