package server

import (
	"io/ioutil"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

// TestNewLibrarian checks that we can create a new instance, close it, and create it again as
// expected.
func TestNewLibrarian(t *testing.T) {
	l1 := newTestLibrarian()
	go func() { <-l1.stop }() // dummy stop signal acceptor

	nodeID1 := l1.selfID // should have been generated
	err := l1.Close()
	assert.Nil(t, err)

	l2, err := NewLibrarian(l1.config, NewDevInfoLogger())
	go func() { <-l2.stop }() // dummy stop signal acceptor

	assert.Nil(t, err)
	assert.Equal(t, nodeID1, l2.selfID)
	err = l2.CloseAndRemove()
	assert.Nil(t, err)
}

func newTestLibrarian() *Librarian {
	config := newTestConfig()
	l, err := NewLibrarian(config, NewDevInfoLogger())
	if err != nil {
		panic(err)
	}
	return l
}

func newTestConfig() *Config {
	config := NewDefaultConfig()
	dir, err := ioutil.TempDir("", "test-data-dir")
	if err != nil {
		panic(err)
	}
	config.WithDataDir(dir).WithDefaultDBDir() // resets default DB dir given new data dir
	return config
}

// alwaysRequestVerifies implements the RequestVerifier interface but just blindly verifies every
// request.
type alwaysRequestVerifier struct{}

func (av *alwaysRequestVerifier) Verify(ctx context.Context, msg proto.Message,
	meta *api.RequestMetadata) error {
	return nil
}

// TestLibrarian_Ping verifies that we receive the expected response ("pong") to a ping request.
func TestLibrarian_Ping(t *testing.T) {
	lib := &Librarian{}
	r, err := lib.Ping(nil, &api.PingRequest{})
	assert.Nil(t, err)
	assert.Equal(t, r.Message, "pong")
}

// TestLibrarian_Identify verifies that we get the expected response from a an identification
// request.
func TestLibrarian_Introduce_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerName, serverPeerIdx := "server", 0
	publicAddr := peer.NewTestPublicAddr(serverPeerIdx)
	rt, serverID, _ := routing.NewTestWithPeers(rng, 128)

	lib := &Librarian{
		config: &Config{
			PublicName: peerName,
			LocalAddr:  publicAddr,
		},
		apiSelf: api.FromAddress(serverID.ID(), peerName, publicAddr),
		fromer:  peer.NewFromer(),
		selfID:  serverID,
		rt:      rt,
		rqv:     &alwaysRequestVerifier{},
	}

	clientID, clientPeerIdx := ecid.NewPseudoRandom(rng), 1
	client := peer.New(
		clientID.ID(),
		"client",
		peer.NewTestConnector(clientPeerIdx),
	)
	assert.Nil(t, lib.rt.Get(client.ID()))

	numPeers := uint32(8)
	rq := &api.IntroduceRequest{
		Metadata: newTestRequestMetadata(rng, clientID),
		Self:     client.ToAPI(),
		NumPeers: numPeers,
	}
	rp, err := lib.Introduce(nil, rq)

	// check response
	assert.Nil(t, err)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	assert.Equal(t, serverID.ID().Bytes(), rp.Self.PeerId)
	assert.Equal(t, peerName, rp.Self.PeerName)
	assert.Equal(t, int(numPeers), len(rp.Peers))

	// check client peer in lib's routing table
	assert.NotNil(t, lib.rt.Get(client.ID()))
}

func TestLibrarian_Introduce_checkRequestErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{}
	rq := &api.IntroduceRequest{
		Metadata: client.NewRequestMetadata(ecid.NewPseudoRandom(rng)),
	}
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Introduce(nil, rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

func TestLibrarian_Introduce_peerIDErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt, _, _ := routing.NewTestWithPeers(rng, 0)

	lib := &Librarian{
		fromer: peer.NewFromer(),
		rt:     rt,
		rqv:    &alwaysRequestVerifier{},
	}

	clientID, clientPeerIdx := ecid.NewPseudoRandom(rng), 1
	client := peer.New(
		clientID.ID(),
		"client",
		peer.NewTestConnector(clientPeerIdx),
	)
	assert.Nil(t, lib.rt.Get(client.ID()))

	// request improperly signed with different public key
	rq := &api.IntroduceRequest{
		Metadata: newTestRequestMetadata(rng, ecid.NewPseudoRandom(rng)),
		Self:     client.ToAPI(),
	}
	rp, err := lib.Introduce(nil, rq)

	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

func TestLibrarian_Find(t *testing.T) {
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	for n := 8; n <= 128; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs

			rng := rand.New(rand.NewSource(int64(s)))
			rt, peerID, nAdded := routing.NewTestWithPeers(rng, n)
			l := &Librarian{
				selfID:     peerID,
				documentSL: storage.NewDocumentKVDBStorerLoader(kvdb),
				kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
				rt:         rt,
				rqv:        &alwaysRequestVerifier{},
			}

			numClosest := uint32(routing.DefaultMaxActivePeers)
			rq := &api.FindRequest{
				Metadata: newTestRequestMetadata(rng, l.selfID),
				Key:      cid.NewPseudoRandom(rng).Bytes(),
				NumPeers: numClosest,
			}

			rp, err := l.Find(nil, rq)
			assert.Nil(t, err)

			// check
			checkPeersResponse(t, rq, rp, nAdded, numClosest)
		}
	}
}

func checkPeersResponse(t *testing.T, rq *api.FindRequest, rp *api.FindResponse, nAdded int,
	numClosest uint32) {

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

func TestLibrarian_Find_present(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	rt, peerID, _ := routing.NewTestWithPeers(rng, 64)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)

	l := &Librarian{
		selfID:     peerID,
		db:         kvdb,
		serverSL:   storage.NewServerKVDBStorerLoader(kvdb),
		documentSL: storage.NewDocumentKVDBStorerLoader(kvdb),
		rt:         rt,
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rqv:        &alwaysRequestVerifier{},
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
	rp, err := l.Find(nil, rq)
	assert.Nil(t, err)

	// we should get back the value we stored
	assert.NotNil(t, rp.Value)
	assert.Nil(t, rp.Peers)
	assert.Equal(t, value, rp.Value)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Find_missing(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	rt, peerID, nAdded := routing.NewTestWithPeers(rng, 64)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)

	l := &Librarian{
		selfID:     peerID,
		rt:         rt,
		db:         kvdb,
		serverSL:   storage.NewServerKVDBStorerLoader(kvdb),
		documentSL: storage.NewDocumentKVDBStorerLoader(kvdb),
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rqv:        &alwaysRequestVerifier{},
	}

	// make request
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.FindRequest{
		Metadata: newTestRequestMetadata(rng, l.selfID),
		Key:      cid.NewPseudoRandom(rng).Bytes(),
		NumPeers: numClosest,
	}

	rp, err := l.Find(nil, rq)
	assert.Nil(t, err)

	// should get peers since the value is missing
	checkPeersResponse(t, rq, rp, nAdded, numClosest)
}

func TestLibrarian_Find_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{}
	rq := client.NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), uint(8))
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Find(nil, rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

func TestLibrarian_Store_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	rt, peerID, _ := routing.NewTestWithPeers(rng, 64)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)

	l := &Librarian{
		selfID:     peerID,
		rt:         rt,
		db:         kvdb,
		serverSL:   storage.NewServerKVDBStorerLoader(kvdb),
		documentSL: storage.NewDocumentKVDBStorerLoader(kvdb),
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:        storage.NewHashKeyValueChecker(),
		rqv:        &alwaysRequestVerifier{},
	}

	// create key-value
	value, key := api.NewTestDocument(rng)

	// make store request
	rq := &api.StoreRequest{
		Metadata: newTestRequestMetadata(rng, l.selfID),
		Key:      key.Bytes(),
		Value:    value,
	}
	rp, err := l.Store(nil, rq)
	assert.Nil(t, err)
	assert.NotNil(t, rp)

	stored, err := l.documentSL.Load(key)
	assert.Nil(t, err)
	assert.Equal(t, value, stored)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func newTestRequestMetadata(rng *rand.Rand, peerID ecid.ID) *api.RequestMetadata {
	return &api.RequestMetadata{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		PubKey:    ecid.ToPublicKeyBytes(peerID),
	}
}

func TestLibrarian_Store_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{}
	value, key := api.NewTestDocument(rng)
	rq := client.NewStoreRequest(ecid.NewPseudoRandom(rng), key, value)
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Store(nil, rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

type errDocStorerLoader struct {}

func (* errDocStorerLoader) Store(key cid.ID, value *api.Document) error {
	return errors.New("some store error")
}

func (* errDocStorerLoader) Load(key cid.ID) (*api.Document, error) {
	return nil, errors.New("some load error")
}

func TestLibrarian_Store_storeError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt, peerID, _ := routing.NewTestWithPeers(rng, 64)
	l := &Librarian{
		selfID: peerID,
		rt: rt,
		kc:         storage.NewExactLengthChecker(storage.EntriesKeyLength),
		kvc:        storage.NewHashKeyValueChecker(),
		rqv:        &alwaysRequestVerifier{},
		documentSL: &errDocStorerLoader{},
	}
	value, key := api.NewTestDocument(rng)
	rq := client.NewStoreRequest(ecid.NewPseudoRandom(rng), key, value)

	rp, err := l.Store(nil, rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
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
	rp, err := l.Get(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, value, rp.Value)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Get_FoundClosestPeers(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := cid.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result to return FoundClosestPeers() == true
	searchParams := search.NewDefaultParameters()
	foundClosestPeersResult := search.NewInitialResult(key, searchParams)
	dummyClosest := peer.NewTestPeers(rng, int(searchParams.NClosestResponses))
	err := foundClosestPeersResult.Closest.SafePushMany(dummyClosest)
	assert.Nil(t, err)

	// create librarian and request
	l := newGetLibrarian(rng, foundClosestPeersResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since fixedSearcher returns a Search value where FoundClosestPeers() is true, shouldn't
	// have any Value
	rp, err := l.Get(nil, rq)
	assert.Nil(t, err)
	assert.Nil(t, rp.Value)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Get_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := cid.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result with fatal error, making Errored() true
	searchParams := search.NewDefaultParameters()
	fatalErrorResult := search.NewInitialResult(key, searchParams)
	fatalErrorResult.FatalErr = errors.New("some fatal error")

	// create librarian and request
	l := newGetLibrarian(rng, fatalErrorResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since we have a fatal search error, Get() should also return an error
	rp, err := l.Get(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Get_Exhausted(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := cid.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result with Exhausted() true
	searchParams := search.NewDefaultParameters()
	exhaustedResult := search.NewInitialResult(key, searchParams)

	// create librarian and request
	l := newGetLibrarian(rng, exhaustedResult, nil)
	rq := client.NewGetRequest(peerID, key)

	// since we have a fatal search error, Get() should also return an error
	rp, err := l.Get(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Get_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := cid.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create librarian and request
	l := newGetLibrarian(rng, nil, errors.New("some unexpected search error"))
	rq := client.NewGetRequest(peerID, key)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Get(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Get_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{}
	rq := client.NewGetRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng))
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Get(nil, rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

func newGetLibrarian(rng *rand.Rand, searchResult *search.Result, searchErr error) *Librarian {
	n := 8
	rt, peerID, _ := routing.NewTestWithPeers(rng, n)
	return &Librarian{
		selfID: peerID,
		config: NewDefaultConfig(),
		rt:     rt,
		kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
		searcher: &fixedSearcher{
			result: searchResult,
			err:    searchErr,
		},
		rqv: &alwaysRequestVerifier{},
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
	rp, err := l.Put(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, uint32(nReplicas), rp.NReplicas)
	assert.Equal(t, api.PutOperation_STORED, rp.Operation)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
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
	rp, err := l.Put(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, api.PutOperation_LEFT_EXISTING, rp.Operation)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
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
	rp, err := l.Put(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Put_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)

	// create librarian and request
	l := newPutLibrarian(rng, nil, errors.New("some store error"))
	rq := client.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Put_checkRequestError(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	l := &Librarian{}
	value, key := api.NewTestDocument(rng)
	rq := client.NewPutRequest(ecid.NewPseudoRandom(rng), key, value)
	rq.Metadata.PubKey = []byte("corrupted pub key")

	rp, err := l.Put(nil, rq)
	assert.Nil(t, rp)
	assert.NotNil(t, err)
}

func newPutLibrarian(rng *rand.Rand, storeResult *store.Result, searchErr error) *Librarian {
	n := 8
	rt, peerID, _ := routing.NewTestWithPeers(rng, n)
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
		rqv: &alwaysRequestVerifier{},
	}
}
