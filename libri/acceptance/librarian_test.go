// +build acceptance

package acceptance

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	lclient "github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server"
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
	verifyName    = "Verify"
)

// things to add later
// - random peer disconnects and additions
// - bad puts and gets
// - data persists after bouncing entire cluster

func TestLibrarianCluster(t *testing.T) {
	params := &params{
		nSeeds:         3,
		nPeers:         32,
		nAuthors:       3,
		logLevel:       zapcore.InfoLevel,
		nIntroductions: 32,
		nPuts:          128,
		nUploads:       16,

		// these are very long, but we want to be robust to shortlived network disruptions in CI
		getTimeout: 20 * time.Second,
		putTimeout: 20 * time.Second,
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

	// check that every peer received the publications of those documents
	checkPublications(t, params, state)

	// down the same ones
	testDownload(t, params, state)

	// share the uploaded docs with other author and download
	testShare(t, params, state)

	// delete some peers and confirm that their docs get replicated
	testReplicate(t, params, state)

	tearDown(state)

	writeBenchmarkResults(t, state.benchResults)
}

func testIntroduce(t *testing.T, params *params, state *state) {
	nPeers := len(state.peers)
	ic := lclient.NewIntroducerCreator(state.clients)
	benchResults := make([]testing.BenchmarkResult, params.nIntroductions)
	fromer := peer.NewFromer()

	// introduce oneself to a number of peers and ensure that each returns the requisite
	// number of new peers
	for c := 0; c < params.nIntroductions; c++ {

		// issue Introduce query to random peer
		i := state.rng.Int31n(int32(nPeers))
		rq := lclient.NewIntroduceRequest(state.client.selfID, state.client.selfAPI, 8)
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			search.DefaultQueryTimeout)
		assert.Nil(t, err)
		introducer, err := ic.Create(state.peerConfigs[i].PublicAddr.String())
		assert.Nil(t, err)
		start := time.Now()
		rp, err := introducer.Introduce(ctx, rq)
		cancel()
		benchResults[c] = testing.BenchmarkResult{
			N: 1,
			T: time.Now().Sub(start),
		}

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, int(rq.NumPeers), len(rp.Peers))

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
	rlc := lclient.NewRetryPutter(putters, params.putTimeout)

	// create a bunch of random putDocs to put
	for c := 0; c < params.nPuts; c++ {

		value, key := newTestDocument(state.rng, putPageSize)
		putDocs[c] = value
		rq := lclient.NewPutRequest(state.client.selfID, key, value)

		// issue Put query to random peer
		ctx, cancel, err := lclient.NewSignedTimeoutContext(state.client.signer, rq,
			store.DefaultQueryTimeout)
		assert.Nil(t, err)
		start := time.Now()
		state.client.logger.Debug("issuing Put request", zap.String("key", key.String()))
		rp, err := rlc.Put(ctx, rq)
		cancel()
		benchResults[c] = testing.BenchmarkResult{
			N:     1,
			T:     time.Now().Sub(start),
			Bytes: int64(putPageSize),
		}
		if rp != nil {
			state.client.logger.Debug("received Put response",
				zap.String("operation", rp.Operation.String()),
				zap.Int("n_replicas", int(rp.NReplicas)),
			)
		}

		// check everything went fine
		assert.Nil(t, err)
		assert.Equal(t, api.PutOperation_STORED, rp.Operation)
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
	rlc := lclient.NewRetryGetter(getters, true, params.getTimeout)

	// create a bunch of random values to put
	for c := 0; c < len(state.putDocs); c++ {

		// create Get request for value
		value := state.putDocs[c]
		key, err := api.GetKey(value)
		assert.Nil(t, err)
		rq := lclient.NewGetRequest(state.client.selfID, key)

		ctx, err := lclient.NewSignedContext(state.client.signer, rq)
		assert.Nil(t, err)
		state.client.logger.Debug("issuing Get request", zap.String("key", key.String()))
		start := time.Now()
		rp, err := rlc.Get(ctx, rq)
		benchResults[c] = testing.BenchmarkResult{
			N:     1,
			T:     time.Now().Sub(start),
			Bytes: int64(putPageSize),
		}
		state.client.logger.Debug("received Get response", zap.String("key", key.String()))

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

	receiveWaitTime := 3 * time.Second
	state.logger.Info("waiting for librarians to receive publications",
		zap.Float64("n_seconds", receiveWaitTime.Seconds()),
	)
	time.Sleep(receiveWaitTime)

	// very occasionally CI network issues can cause almost all of the peers to miss a publication;
	// we don't want to break everything when this happens
	acceptableNMissing := 2

	// check all peers have publications for all docs
	for i, p := range state.peers {
		info := fmt.Sprintf("peer %d, nRecenptPubs: %d", i, p.RecentPubs.Len())
		assert.True(t, p.RecentPubs.Len() >= params.nUploads-acceptableNMissing, info)
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

func testReplicate(t *testing.T, _ *params, state *state) {

	// take n peers out of the network
	toRemove := state.peers[:8]
	state.peers = state.peers[8:]
	for _, p1 := range toRemove {
		go func(p2 *server.Librarian) {
			// explicitly end subscriptions first and then sleep so that later librarians
			// don't crash b/c of flurry of ended subscriptions from earlier librarians
			p2.StopAuxRoutines()
			time.Sleep(3 * time.Second)
			err := p2.Close()
			assert.Nil(t, err)
		}(p1)
	}

	// verify each doc, seeing which are under-replicated
	nReplicas1 := countDocReplicas(t, state)
	nUnderReplicated1 := 0
	for _, keyNReplicas := range nReplicas1 {
		if keyNReplicas < int(store.DefaultNReplicas) {
			nUnderReplicated1++
		}
	}
	assert.True(t, nUnderReplicated1 > 0)
	state.logger.Info("finished replica audit 1", zap.Int("n_under_replicated", nUnderReplicated1))

	rereplicateWaitSecs := 10
	state.logger.Info("waiting for re-replication to happen",
		zap.Int("n_secs", rereplicateWaitSecs),
	)
	time.Sleep(time.Duration(rereplicateWaitSecs) * time.Second)

	nReplicas2 := countDocReplicas(t, state)
	nUnderReplicated2 := 0
	for _, keyNReplicas := range nReplicas2 {
		if keyNReplicas < int(store.DefaultNReplicas) {
			nUnderReplicated2++
			break
		}
	}
	info := fmt.Sprintf("nUnderReplicated1: %d, nUnderReplicated2: %d", nUnderReplicated1,
		nUnderReplicated2)
	assert.True(t, nUnderReplicated2 < nUnderReplicated1, info)
	state.logger.Info("finished replica audit 2", zap.Int("n_under_replicated", nUnderReplicated2))
}

func countDocReplicas(t *testing.T, state *state) map[string]int {
	benchResults := make([]testing.BenchmarkResult, len(state.putDocs))
	verifier := verify.NewDefaultVerifier(client.NewSigner(state.client.selfID.Key()),
		state.clients)
	verifyParams := verify.NewDefaultParameters()

	nReplicas := make(map[string]int)
	for i, doc := range state.putDocs {
		key, err := api.GetKey(doc)
		assert.Nil(t, err)
		macKey := make([]byte, 32)
		_, err = rand.Read(macKey)
		assert.Nil(t, err)
		docBytes, err := proto.Marshal(doc)
		assert.Nil(t, err)

		start := time.Now()
		seeds := state.client.rt.Peak(key, verifyParams.NClosestResponses)
		v := verify.NewVerify(state.client.selfID, key, docBytes, macKey, verifyParams)
		err = verifier.Verify(v, seeds)
		if err == nil {
			// don't fail if a verification occasionally errors
			nReplicas[key.String()] = len(v.Result.Replicas)
		}
		benchResults[i] = testing.BenchmarkResult{
			N: 1,
			T: time.Now().Sub(start),
		}
	}
	state.benchResults = append(state.benchResults, &benchmarkObs{
		name:    verifyName,
		procs:   runtime.NumCPU(),
		results: benchResults,
	})
	return nReplicas
}
