package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/common/subscribe"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	gw "github.com/drausin/libri/libri/librarian/server/goodwill"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"
)

// TestNewLibrarian checks that we can create a new instance, close it, and create it again as
// expected.
func TestNewLibrarian(t *testing.T) {
	l1 := newTestLibrarian()
	go func() {
		err := l1.replicator.Start()
		assert.Nil(t, err)
	}()
	go func() { // dummy stop signal acceptor
		<-l1.stop
		close(l1.stopped)
	}()

	nodeID1 := l1.selfID // should have been generated
	err := l1.Close()
	assert.Nil(t, err)

	l2, err := NewLibrarian(l1.config, zap.NewNop())
	go func() {
		err2 := l2.replicator.Start()
		assert.Nil(t, err2)
	}()
	go func() { // dummy stop signal acceptor
		<-l2.stop
		close(l2.stopped)
	}()

	assert.Nil(t, err)
	assert.Equal(t, nodeID1, l2.selfID)
	err = l2.CloseAndRemove()
	assert.Nil(t, err)
}

func newTestLibrarian() *Librarian {
	config := newTestConfig()
	l, err := NewLibrarian(config, zap.NewNop())
	cerrors.MaybePanic(err)
	return l
}

func newTestConfig() *Config {
	config := NewDefaultConfig()
	dir, err := ioutil.TempDir("", "test-data-dir")
	cerrors.MaybePanic(err)
	config.WithDataDir(dir).WithDefaultDBDir() // resets default DB dir given new data dir
	config.WithReportMetrics(false)            // otherwise can get duplicate Prom registrations
	return config
}

// alwaysRequestVerifies implements the RequestVerifier interface but just blindly verifies every
// request.
type alwaysRequestVerifier struct{}

func (av *alwaysRequestVerifier) Verify(ctx context.Context, msg proto.Message,
	meta *api.RequestMetadata) error {
	return nil
}

func TestLibrarian_Introduce_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerName, serverPeerIdx := "server", 0
	publicAddr := peer.NewTestPublicAddr(serverPeerIdx)
	rt, serverID, _, _ := routing.NewTestWithPeers(rng, 128)
	rec := gw.NewScalarRecorder()

	lib := &Librarian{
		config: &Config{
			PublicName: peerName,
			LocalPort:  publicAddr.Port,
		},
		apiSelf: peer.FromAddress(serverID.ID(), peerName, publicAddr),
		fromer:  peer.NewFromer(),
		selfID:  serverID,
		rt:      rt,
		rqv:     &alwaysRequestVerifier{},
		rec:     rec,
		logger:  zap.NewNop(), // clogging.NewDevInfoLogger(),
	}

	clientID, clientPeerIdx := ecid.NewPseudoRandom(rng), 1
	clientImpl := peer.New(
		clientID.ID(),
		"client",
		peer.NewTestPublicAddr(clientPeerIdx),
	)
	clientImpl2, exists := lib.rt.Get(clientImpl.ID())
	assert.False(t, exists)
	assert.Nil(t, clientImpl2)

	numPeers := uint32(8)
	rq := &api.IntroduceRequest{
		Metadata: newTestRequestMetadata(rng, clientID),
		Self:     clientImpl.ToAPI(),
		NumPeers: numPeers,
	}
	rp, err := lib.Introduce(context.Background(), rq)

	// check response
	assert.Nil(t, err)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	assert.Equal(t, serverID.ID().Bytes(), rp.Self.PeerId)
	assert.Equal(t, peerName, rp.Self.PeerName)
	assert.Equal(t, int(numPeers), len(rp.Peers))
	qo := rec.Get(clientImpl.ID(), api.Introduce)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Introduce_checkRequestErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rec := gw.NewScalarRecorder()
	l := &Librarian{
		logger: zap.NewNop(), // clogging.NewDevInfoLogger()
		rec:    rec,
	}
	rq := &api.IntroduceRequest{
		Metadata: client.NewRequestMetadata(ecid.NewPseudoRandom(rng)),
	}
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Introduce(context.Background(), rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

func TestLibrarian_Introduce_peerIDErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt, _, _, _ := routing.NewTestWithPeers(rng, 0)
	rec := gw.NewScalarRecorder()

	lib := &Librarian{
		fromer: peer.NewFromer(),
		rt:     rt,
		rqv:    &alwaysRequestVerifier{},
		rec:    rec,
		logger: zap.NewNop(), // clogging.NewDevInfoLogger()
	}

	clientID, clientPeerIdx := ecid.NewPseudoRandom(rng), 1
	client1 := peer.New(
		clientID.ID(),
		"client",
		peer.NewTestPublicAddr(clientPeerIdx),
	)
	client2, exists := lib.rt.Get(client1.ID())
	assert.Nil(t, client2)
	assert.False(t, exists)

	// request improperly signed with different public key
	otherID := ecid.NewPseudoRandom(rng)
	rq := &api.IntroduceRequest{
		Metadata: newTestRequestMetadata(rng, otherID),
		Self:     client1.ToAPI(),
	}
	rp, err := lib.Introduce(context.Background(), rq)

	assert.Nil(t, rp)
	assert.NotNil(t, err)
	qo := rec.Get(otherID.ID(), api.Introduce)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Error].Count))
}

func TestLibrarian_Find_peers(t *testing.T) {
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	for n := 8; n <= 128; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs

			rng := rand.New(rand.NewSource(int64(s)))
			rt, peerID, nAdded, _ := routing.NewTestWithPeers(rng, n)
			rec := gw.NewScalarRecorder()
			l := &Librarian{
				selfID:     peerID,
				documentSL: storage.NewDocumentSLD(kvdb),
				kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
				rt:         rt,
				rqv:        &alwaysRequestVerifier{},
				rec:        rec,
				logger:     zap.NewNop(), // clogging.NewDevInfoLogger()
			}

			numClosest := uint32(routing.DefaultMaxActivePeers)
			rq := &api.FindRequest{
				Metadata: newTestRequestMetadata(rng, l.selfID),
				Key:      id.NewPseudoRandom(rng).Bytes(),
				NumPeers: numClosest,
			}

			rp, err := l.Find(context.Background(), rq)
			assert.Nil(t, err)

			// check
			checkPeersFindResponse(t, rq, rp, nAdded, numClosest)
			qo := rec.Get(l.selfID.ID(), api.Find)
			assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
		}
	}
}

func checkPeersFindResponse(
	t *testing.T, rq *api.FindRequest, rp *api.FindResponse, nAdded int, numClosest uint32,
) {

	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)

	assert.Nil(t, rp.Value)
	assert.NotNil(t, rp.Peers)
	if int(numClosest) > nAdded {
		assert.Equal(t, nAdded, len(rp.Peers))
	} else {
		assert.Equal(t, numClosest, uint32(len(rp.Peers)))
	}
	for _, a := range rp.Peers {
		assert.NotNil(t, a.PeerId)
		assert.NotNil(t, a.PeerName)
		assert.NotNil(t, a.Ip)
		assert.NotNil(t, a.Port)
	}
}

func TestLibrarian_Find_value(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, 64)
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)

	rec := gw.NewScalarRecorder()
	l := &Librarian{
		selfID:     peerID,
		db:         kvdb,
		serverSL:   storage.NewServerSL(kvdb),
		documentSL: storage.NewDocumentSLD(kvdb),
		rt:         rt,
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rqv:        &alwaysRequestVerifier{},
		rec:        rec,
		logger:     zap.NewNop(), // clogging.NewDevInfoLogger()
	}

	// create key-value and store
	value, key := api.NewTestDocument(rng)
	err = l.documentSL.Store(key, value)
	assert.Nil(t, err)

	// make request for key
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.FindRequest{
		Metadata: newTestRequestMetadata(rng, l.selfID),
		Key:      key.Bytes(),
		NumPeers: numClosest,
	}
	rp, err := l.Find(context.Background(), rq)
	assert.Nil(t, err)
	qo := rec.Get(l.selfID.ID(), api.Find)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))

	// we should get back the value we stored
	assert.NotNil(t, rp.Value)
	assert.Nil(t, rp.Peers)
	assert.Equal(t, value, rp.Value)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Find_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	rt, _, _, _ := routing.NewTestWithPeers(rng, 0)
	rec := gw.NewScalarRecorder()
	cases := []struct {
		l         *Librarian
		rqCreator func() *api.FindRequest
	}{
		// case 0
		{
			l: &Librarian{
				logger: zap.NewNop(), // clogging.NewDevInfoLogger()
				rqv:    NewRequestVerifier(),
				kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
				rec:    rec,
			},
			rqCreator: func() *api.FindRequest {
				rq := client.NewFindRequest(peerID, key, uint(8))
				rq.Metadata.PubKey = []byte("corrupted pub key")
				return rq
			},
		},

		// case 1
		{
			l: &Librarian{
				logger:     zap.NewNop(), // clogging.NewDevInfoLogger()
				rqv:        &alwaysRequestVerifier{},
				kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
				documentSL: &storage.TestDocSLD{LoadErr: errors.New("some Load error")},
				rec:        rec,
				rt:         rt,
			},
			rqCreator: func() *api.FindRequest {
				return client.NewFindRequest(peerID, key, uint(8))
			},
		},
	}

	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp, err := c.l.Find(context.Background(), c.rqCreator())
		assert.Nil(t, rp, info)
		assert.NotNil(t, err, info)
	}
}

func TestLibrarian_Verify_value(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, 64)
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)

	rec := gw.NewScalarRecorder()
	l := &Librarian{
		selfID:     peerID,
		db:         kvdb,
		serverSL:   storage.NewServerSL(kvdb),
		documentSL: storage.NewDocumentSLD(kvdb),
		rt:         rt,
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rqv:        &alwaysRequestVerifier{},
		rec:        rec,
		logger:     zap.NewNop(), // clogging.NewDevInfoLogger(),
	}

	// create key-value and store
	value, key := api.NewTestDocument(rng)
	err = l.documentSL.Store(key, value)
	assert.Nil(t, err)

	// MAC the value
	macKey := api.RandBytes(rng, 32)
	macer := hmac.New(sha256.New, macKey)
	valueBytes, err := proto.Marshal(value)
	assert.Nil(t, err)
	_, err = macer.Write(valueBytes)
	assert.Nil(t, err)
	expectedMAC := macer.Sum(nil)

	// make request for key
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.VerifyRequest{
		Metadata: newTestRequestMetadata(rng, l.selfID),
		Key:      key.Bytes(),
		MacKey:   macKey,
		NumPeers: numClosest,
	}
	rp, err := l.Verify(context.Background(), rq)
	assert.Nil(t, err)

	// we should get back the expected mac
	assert.Equal(t, expectedMAC, rp.Mac)
	assert.Nil(t, rp.Peers)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	qo := rec.Get(l.selfID.ID(), api.Verify)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Verify_peers(t *testing.T) {
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)
	for n := 8; n <= 128; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs

			rng := rand.New(rand.NewSource(int64(s)))
			rt, peerID, nAdded, _ := routing.NewTestWithPeers(rng, n)
			rec := gw.NewScalarRecorder()
			l := &Librarian{
				selfID:     peerID,
				documentSL: storage.NewDocumentSLD(kvdb),
				kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
				rt:         rt,
				rqv:        &alwaysRequestVerifier{},
				rec:        rec,
				logger:     zap.NewNop(), // clogging.NewDevInfoLogger(),
			}

			numClosest := uint32(routing.DefaultMaxActivePeers)
			rq := &api.VerifyRequest{
				Metadata: newTestRequestMetadata(rng, l.selfID),
				Key:      id.NewPseudoRandom(rng).Bytes(),
				NumPeers: numClosest,
			}

			rp, err := l.Verify(context.Background(), rq)
			assert.Nil(t, err)

			// check
			checkPeersVerifyResponse(t, rq, rp, nAdded, numClosest)
			qo := rec.Get(l.selfID.ID(), api.Verify)
			assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
		}
	}
}

func checkPeersVerifyResponse(
	t *testing.T, rq *api.VerifyRequest, rp *api.VerifyResponse, nAdded int, numClosest uint32,
) {
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)

	assert.Nil(t, rp.Mac)
	assert.NotNil(t, rp.Peers)
	if int(numClosest) > nAdded {
		assert.Equal(t, nAdded, len(rp.Peers))
	} else {
		assert.Equal(t, numClosest, uint32(len(rp.Peers)))
	}
	for _, a := range rp.Peers {
		assert.NotNil(t, a.PeerId)
		assert.NotNil(t, a.PeerName)
		assert.NotNil(t, a.Ip)
		assert.NotNil(t, a.Port)
	}
}

func TestLibrarian_Verify_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	rt, _, _, _ := routing.NewTestWithPeers(rng, 0)
	rec := gw.NewScalarRecorder()
	macKey := api.RandBytes(rng, 32)
	cases := []struct {
		l         *Librarian
		rqCreator func() *api.VerifyRequest
	}{
		// case 0
		{
			l: &Librarian{
				logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
				rqv:    NewRequestVerifier(),
				kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
				rec:    rec,
			},
			rqCreator: func() *api.VerifyRequest {
				rq := client.NewVerifyRequest(peerID, key, macKey, uint(8))
				rq.Metadata.PubKey = []byte("corrupted pub key")
				return rq
			},
		},

		// case 1
		{
			l: &Librarian{
				logger:     zap.NewNop(), // clogging.NewDevInfoLogger(),
				rqv:        &alwaysRequestVerifier{},
				kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
				documentSL: &storage.TestDocSLD{MacErr: errors.New("some Mac error")},
				rt:         rt,
				rec:        rec,
			},
			rqCreator: func() *api.VerifyRequest {
				return client.NewVerifyRequest(peerID, key, macKey, uint(8))
			},
		},
	}

	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp, err := c.l.Verify(context.Background(), c.rqCreator())
		assert.Nil(t, rp, info)
		assert.NotNil(t, err, info)
	}
}

func TestLibrarian_Store_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, 64)
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	defer cleanup()
	defer kvdb.Close()
	assert.Nil(t, err)

	rec := gw.NewScalarRecorder()
	l := &Librarian{
		selfID:         peerID,
		rt:             rt,
		db:             kvdb,
		serverSL:       storage.NewServerSL(kvdb),
		documentSL:     storage.NewDocumentSLD(kvdb),
		subscribeTo:    &fixedTo{},
		kc:             storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:            storage.NewHashKeyValueChecker(),
		rqv:            &alwaysRequestVerifier{},
		storageMetrics: newStorageMetrics(),
		rec:            rec,
		logger:         zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	defer l.storageMetrics.unregister()

	// create key-value
	value, key := api.NewTestDocument(rng)

	// make store request
	rq := &api.StoreRequest{
		Metadata: newTestRequestMetadata(rng, l.selfID),
		Key:      key.Bytes(),
		Value:    value,
	}
	rp, err := l.Store(context.Background(), rq)
	assert.Nil(t, err)
	assert.NotNil(t, rp)

	stored, err := l.documentSL.Load(key)
	assert.Nil(t, err)
	assert.Equal(t, value, stored)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	qo := rec.Get(l.selfID.ID(), api.Store)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func newTestRequestMetadata(rng *rand.Rand, peerID ecid.ID) *api.RequestMetadata {
	return &api.RequestMetadata{
		RequestId: id.NewPseudoRandom(rng).Bytes(),
		PubKey:    peerID.PublicKeyBytes(),
	}
}

func TestLibrarian_Store_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
		rec:    gw.NewScalarRecorder(),
	}
	value, key := api.NewTestDocument(rng)
	rqID := ecid.NewPseudoRandom(rng)
	rq := client.NewStoreRequest(rqID, key, value)
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Store(context.Background(), rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
	qo := l.rec.Get(rqID.ID(), api.Get)
	assert.Equal(t, 0, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Store_storeError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, 64)
	sld := storage.NewTestDocSLD()
	sld.StoreErr = errors.New("some Store error")
	rec := gw.NewScalarRecorder()
	l := &Librarian{
		selfID:     peerID,
		rt:         rt,
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:        storage.NewHashKeyValueChecker(),
		rqv:        &alwaysRequestVerifier{},
		documentSL: sld,
		rec:        rec,
		logger:     zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	value, key := api.NewTestDocument(rng)
	rqID := ecid.NewPseudoRandom(rng)
	rq := client.NewStoreRequest(rqID, key, value)

	rp, err := l.Store(context.Background(), rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
	qo := rec.Get(rqID.ID(), api.Store)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

type fixedSearcher struct {
	result *search.Result
	err    error
}

func (s *fixedSearcher) Search(search *search.Search, seeds []peer.Peer) error {
	if s.err != nil {
		return s.err
	}
	search.Result = s.result
	return nil
}

func TestLibrarian_Get_FoundValue(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been found
	searchParams := search.NewDefaultParameters()
	foundValueResult := search.NewInitialResult(key, searchParams)
	foundValueResult.Value = value

	// create librarian and request
	l := newGetLibrarian(rng, foundValueResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Get(context.Background(), rq)
	assert.Nil(t, err)
	assert.Equal(t, value, rp.Value)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	qo := l.rec.Get(peerID.ID(), api.Get)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Get_FoundClosestPeers(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := id.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result to return UnderReplicated() == true
	searchParams := search.NewDefaultParameters()
	foundClosestPeersResult := search.NewInitialResult(key, searchParams)
	dummyClosest := peer.NewTestPeers(rng, int(searchParams.NClosestResponses))
	foundClosestPeersResult.Closest.SafePushMany(dummyClosest)

	// create librarian and request
	l := newGetLibrarian(rng, foundClosestPeersResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since fixedSearcher returns a Search value where UnderReplicated() is true, shouldn't
	// have any Value
	rp, err := l.Get(context.Background(), rq)
	assert.Nil(t, err)
	assert.Nil(t, rp.Value)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	qo := l.rec.Get(peerID.ID(), api.Get)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Get_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := id.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result with fatal error, making Errored() true
	searchParams := search.NewDefaultParameters()
	fatalErrorResult := search.NewInitialResult(key, searchParams)
	fatalErrorResult.FatalErr = errors.New("some fatal error")

	// create librarian and request
	l := newGetLibrarian(rng, fatalErrorResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since we have a fatal search error, Get() should also return an error
	rp, err := l.Get(context.Background(), rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
	qo := l.rec.Get(peerID.ID(), api.Get)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Get_Exhausted(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := id.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result with Exhausted() true
	searchParams := search.NewDefaultParameters()
	exhaustedResult := search.NewInitialResult(key, searchParams)

	// create librarian and request
	l := newGetLibrarian(rng, exhaustedResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since we have a fatal search error, Get() should also return an error
	rp, err := l.Get(context.Background(), rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
	qo := l.rec.Get(peerID.ID(), api.Get)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Get_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := id.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create librarian and request
	l := newGetLibrarian(rng, nil, errors.New("some unexpected search error"))
	rq := client.NewGetRequest(peerID, key)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Get(context.Background(), rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
	qo := l.rec.Get(peerID.ID(), api.Get)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Get_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
		rec:    gw.NewScalarRecorder(),
	}
	rqID := ecid.NewPseudoRandom(rng)
	rq := client.NewGetRequest(rqID, id.NewPseudoRandom(rng))
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Get(context.Background(), rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
	qo := l.rec.Get(rqID.ID(), api.Get)
	assert.Equal(t, 0, int(qo[gw.Request][gw.Success].Count))
}

func newGetLibrarian(rng *rand.Rand, searchResult *search.Result, searchErr error) *Librarian {
	n := 8
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, n)
	return &Librarian{
		selfID: peerID,
		config: NewDefaultConfig(),
		rt:     rt,
		kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
		searcher: &fixedSearcher{
			result: searchResult,
			err:    searchErr,
		},
		rqv:    &alwaysRequestVerifier{},
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
}

type fixedStorer struct {
	result *store.Result
	err    error
}

func (s *fixedStorer) Store(store *store.Store, seeds []peer.Peer) error {
	if s.err != nil {
		return s.err
	}
	store.Result = s.result
	return nil
}

func TestLibrarian_Put_Stored(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been stored
	searchParams := search.NewDefaultParameters()
	nReplicas := searchParams.NClosestResponses
	addedResult := store.NewInitialResult(search.NewInitialResult(key, searchParams))
	addedResult.Responded = peer.NewTestPeers(rng, int(nReplicas))

	// create librarian and request
	l := newPutLibrarian(rng, addedResult, nil)
	rq := client.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(context.Background(), rq)
	assert.Nil(t, err)
	assert.Equal(t, uint32(nReplicas), rp.NReplicas)
	assert.Equal(t, api.PutOperation_STORED, rp.Operation)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	qo := l.rec.Get(peerID.ID(), api.Put)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Put_Exists(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been stored
	searchParams := search.NewDefaultParameters()
	foundValueResult := search.NewInitialResult(key, searchParams)
	foundValueResult.Value = value
	existsResult := store.NewInitialResult(foundValueResult)

	// create librarian and request
	l := newPutLibrarian(rng, existsResult, nil)
	rq := client.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(context.Background(), rq)
	assert.Nil(t, err)
	assert.Equal(t, api.PutOperation_LEFT_EXISTING, rp.Operation)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	qo := l.rec.Get(peerID.ID(), api.Put)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Put_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been stored
	searchParams := search.NewDefaultParameters()
	erroredResult := store.NewInitialResult(search.NewInitialResult(key, searchParams))
	erroredResult.FatalErr = errors.New("some fatal error")

	// create librarian and request
	l := newPutLibrarian(rng, erroredResult, nil)
	rq := client.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(context.Background(), rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
	qo := l.rec.Get(peerID.ID(), api.Put)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count)) // request was ok
}

func TestLibrarian_Put_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)

	// create librarian and request
	l := newPutLibrarian(rng, nil, errors.New("some store error"))
	rq := client.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(context.Background(), rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
	qo := l.rec.Get(peerID.ID(), api.Put)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count)) // request was ok
}

func TestLibrarian_Put_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	value, key := api.NewTestDocument(rng)
	rqID := ecid.NewPseudoRandom(rng)
	rq := client.NewPutRequest(rqID, key, value)
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Put(context.Background(), rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
	qo := l.rec.Get(rqID.ID(), api.Put)
	assert.Equal(t, 0, int(qo[gw.Request][gw.Success].Count))
}

func TestLibrarian_Subscribe_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nPubs := 64
	newPubs := make(chan *subscribe.KeyedPub)
	done := make(chan struct{})
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, 0)
	l := &Librarian{
		selfID: peerID,
		subscribeFrom: &fixedFrom{
			new:  newPubs,
			done: done,
		},
		rqv:    &alwaysRequestVerifier{},
		rt:     rt,
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}

	// create subscription that should cover both the author and reader filter code branches
	targetFP := float64(0.5)
	filterFP := math.Sqrt(targetFP) // so product of author and reader FP rates is targetFP
	sub, err := subscribe.NewSubscription([][]byte{}, filterFP, [][]byte{}, filterFP, rng)
	assert.Nil(t, err)

	rqID := ecid.NewPseudoRandom(rng)
	rq := client.NewSubscribeRequest(rqID, sub)
	from := &fixedLibrarianSubscribeServer{
		sent: make(chan *api.SubscribeResponse),
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err = l.Subscribe(rq, from)
		assert.Nil(t, err)
		qo := l.rec.Get(rqID.ID(), api.Subscribe)
		assert.Equal(t, 1, int(qo[gw.Request][gw.Success].Count))
	}(wg)

	// generate pubs we're going to send
	newPubsMap := make(map[string]*api.Publication)
	for c := 0; c < nPubs; c++ {
		newPub := newKeyedPub(t, api.NewTestPublication(rng))
		newPubsMap[newPub.Key.String()] = newPub.Value
	}

	// check % pubs sent to client is >= targetFP
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		sentPubs := 0
		for sentPub := range from.sent {
			key := id.FromBytes(sentPub.Key)
			value, in := newPubsMap[key.String()]
			assert.True(t, in)
			assert.Equal(t, value, sentPub.Value)
			sentPubs++
		}
		info := fmt.Sprintf("sentPubs: %d, nPubs: %d", sentPubs, nPubs)
		assert.True(t, int(float64(nPubs)*targetFP) < sentPubs, info)
	}(wg)

	// send pubs
	for _, value := range newPubsMap {
		newPubs <- newKeyedPub(t, value)
	}

	// ensure graceful end of l.Subscribe()
	close(newPubs)
	<-done

	// ensure end to goroutine above that reads from from.sent
	close(from.sent)

	// wait for all goroutines to finish
	wg.Wait()
}

func TestLibrarian_Subscribe_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	sub, err := subscribe.NewFPSubscription(1.0, rng) // get everything
	assert.Nil(t, err)
	rt, selfID, _, _ := routing.NewTestWithPeers(rng, 0)
	rq := client.NewSubscribeRequest(selfID, sub)
	from := &fixedLibrarianSubscribeServer{
		sent: make(chan *api.SubscribeResponse),
	}

	// check request error bubbles up
	l1 := &Librarian{
		rqv:    &neverRequestVerifier{},
		rt:     rt,
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	err = l1.Subscribe(rq, from)
	assert.NotNil(t, err)
	qo := l1.rec.Get(selfID.ID(), api.Subscribe)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Error].Count))

	// check author filter error bubbles up
	sub2, err := subscribe.NewFPSubscription(1.0, rng)
	assert.Nil(t, err)
	sub2.AuthorPublicKeys.Encoded = nil // will trigger error
	rq2 := client.NewSubscribeRequest(selfID, sub2)
	l2 := &Librarian{
		rqv:    &alwaysRequestVerifier{},
		rt:     rt,
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	err = l2.Subscribe(rq2, from)
	assert.NotNil(t, err)
	qo = l2.rec.Get(selfID.ID(), api.Subscribe)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Error].Count))

	// check reader filter error bubbles up
	sub3, err := subscribe.NewFPSubscription(1.0, rng)
	assert.Nil(t, err)
	sub3.ReaderPublicKeys.Encoded = nil // will trigger error
	rq3 := client.NewSubscribeRequest(selfID, sub3)
	l3 := &Librarian{
		rqv:    &alwaysRequestVerifier{},
		rt:     rt,
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	err = l3.Subscribe(rq3, from)
	assert.NotNil(t, err)
	qo = l3.rec.Get(selfID.ID(), api.Subscribe)
	assert.Equal(t, 1, int(qo[gw.Request][gw.Error].Count))

	// check subscribeFrom.New() bubbles up
	sub4, err := subscribe.NewFPSubscription(1.0, rng)
	assert.Nil(t, err)
	rq4 := client.NewSubscribeRequest(selfID, sub4)
	l4 := &Librarian{
		selfID: ecid.NewPseudoRandom(rng),
		subscribeFrom: &fixedFrom{
			err: subscribe.ErrNotAcceptingNewSubscriptions,
		},
		rqv:    &alwaysRequestVerifier{},
		rt:     rt,
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	err = l4.Subscribe(rq4, from)
	assert.Equal(t, subscribe.ErrNotAcceptingNewSubscriptions, err)
	qo = l4.rec.Get(selfID.ID(), api.Subscribe)
	assert.Equal(t, 0, int(qo[gw.Request][gw.Error].Count)) // not rq error

	// check from.Send() error bubbles up
	sub5, err := subscribe.NewFPSubscription(1.0, rng)
	assert.Nil(t, err)
	rq5 := client.NewSubscribeRequest(selfID, sub5)
	newPubs := make(chan *subscribe.KeyedPub)
	l5 := &Librarian{
		selfID: ecid.NewPseudoRandom(rng),
		subscribeFrom: &fixedFrom{
			new:  newPubs,
			done: make(chan struct{}),
		},
		rqv:    &alwaysRequestVerifier{},
		rt:     rt,
		rec:    gw.NewScalarRecorder(),
		logger: zap.NewNop(), // clogging.NewDevInfoLogger(),
	}
	from5 := &fixedLibrarianSubscribeServer{
		err: errors.New("some Subscribe error"),
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err = l5.Subscribe(rq5, from5)
		assert.NotNil(t, err)
		qo = l5.rec.Get(selfID.ID(), api.Subscribe)
		assert.Equal(t, 0, int(qo[gw.Request][gw.Error].Count))
	}(wg)
	newPubs <- newKeyedPub(t, api.NewTestPublication(rng))
	wg.Wait()
}

func newKeyedPub(t *testing.T, pub *api.Publication) *subscribe.KeyedPub {
	key, err := api.GetKey(pub)
	assert.Nil(t, err)
	return &subscribe.KeyedPub{
		Key:   key,
		Value: pub,
	}
}

type fixedFrom struct {
	new  chan *subscribe.KeyedPub
	done chan struct{}
	err  error
}

func (f *fixedFrom) New() (chan *subscribe.KeyedPub, chan struct{}, error) {
	return f.new, f.done, f.err
}

func (f *fixedFrom) Fanout() {}

type fixedTo struct {
	beginErr error
	sendErr  error
}

func (t *fixedTo) Begin() error {
	return t.beginErr
}

func (t *fixedTo) End() {}

func (t *fixedTo) Send(pub *api.Publication) error {
	return t.sendErr
}

type fixedLibrarianSubscribeServer struct {
	sent chan *api.SubscribeResponse
	err  error
}

func (f *fixedLibrarianSubscribeServer) Send(rp *api.SubscribeResponse) error {
	if f.err != nil {
		return f.err
	}
	f.sent <- rp
	return nil
}

// the stubs below are just to satisfy the Librarian_SubscribeServer interface
func (f *fixedLibrarianSubscribeServer) SetHeader(metadata.MD) error {
	return nil
}

func (f *fixedLibrarianSubscribeServer) SendHeader(metadata.MD) error {
	return nil
}

func (f *fixedLibrarianSubscribeServer) SetTrailer(metadata.MD) {}

func (f *fixedLibrarianSubscribeServer) Context() context.Context {
	return nil
}

func (f *fixedLibrarianSubscribeServer) SendMsg(m interface{}) error {
	return nil
}

func (f *fixedLibrarianSubscribeServer) RecvMsg(m interface{}) error {
	return nil
}

func newPutLibrarian(rng *rand.Rand, storeResult *store.Result, searchErr error) *Librarian {
	n := 8
	rt, peerID, _, _ := routing.NewTestWithPeers(rng, n)
	return &Librarian{
		selfID: peerID,
		config: NewDefaultConfig(),
		rt:     rt,
		kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:    storage.NewHashKeyValueChecker(),
		storer: &fixedStorer{
			result: storeResult,
			err:    searchErr,
		},
		rqv:    &alwaysRequestVerifier{},
		rec:    gw.NewScalarRecorder(),
		logger: clogging.NewDevInfoLogger(),
	}
}
