package server

import (
	"bytes"
	"crypto/sha256"
	"io/ioutil"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// TestNewLibrarian checks that we can create a new instance, close it, and create it again as
// expected.
func TestNewLibrarian(t *testing.T) {
	lib1 := newTestLibrarian()

	nodeID1 := lib1.PeerID // should have been generated
	err := lib1.Close()
	assert.Nil(t, err)

	lib2, err := NewLibrarian(lib1.Config)
	assert.Nil(t, err)
	assert.Equal(t, nodeID1, lib2.PeerID)
	err = lib2.Close()
	assert.Nil(t, err)
}

func newTestLibrarian() *Librarian {
	config := DefaultConfig()
	dir, err := ioutil.TempDir("", "test-data-dir")
	if err != nil {
		panic(err)
	}
	config.SetDataDir(dir)

	l, err := NewLibrarian(config)
	if err != nil {
		panic(err)
	}
	return l
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
func TestLibrarian_Identify(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	peerName := "Test Node"
	lib := &Librarian{
		Config: &Config{
			PeerName: peerName,
		},
		PeerID: peerID,
	}

	rq := &api.IdentityRequest{
		Metadata: newTestRequestMetadata(rng, lib.PeerID),
	}
	rp, err := lib.Identify(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	assert.Equal(t, peerName, rp.PeerName)
}

func TestLibrarian_Find(t *testing.T) {
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	for n := 8; n <= 128; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs

			rng := rand.New(rand.NewSource(int64(s)))
			rt, peerID := routing.NewTestWithPeers(rng, n)
			l := &Librarian{
				PeerID:    peerID,
				entriesSL: storage.NewEntriesKVDBStorerLoader(kvdb),
				kc:        storage.NewExactLengthChecker(storage.EntriesKeyLength),
				rt:        rt,
			}

			numClosest := uint32(routing.DefaultMaxActivePeers)
			rq := &api.FindRequest{
				Metadata: newTestRequestMetadata(rng, l.PeerID),
				Key:       cid.NewPseudoRandom(rng).Bytes(),
				NumPeers:  numClosest,
			}

			prevNumActivePeers := l.rt.NumPeers()
			rp, err := l.Find(nil, rq)
			assert.Nil(t, err)

			// check
			checkPeersResponse(t, rq, rp, rt, prevNumActivePeers, numClosest)
		}
	}
}

func checkPeersResponse(t *testing.T, rq *api.FindRequest, rp *api.FindResponse, rt routing.Table,
	prevNumActivePeers uint, numClosest uint32) {

	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
	assert.Equal(t, prevNumActivePeers, rt.NumPeers())

	assert.Nil(t, rp.Value)
	assert.NotNil(t, rp.Addresses)
	if uint(numClosest) > prevNumActivePeers {
		assert.Equal(t, int(prevNumActivePeers), len(rp.Addresses))
	} else {
		assert.Equal(t, numClosest, uint32(len(rp.Addresses)))
	}
	for _, a := range rp.Addresses {
		assert.NotNil(t, a.PeerId)
		assert.NotNil(t, a.PeerName)
		assert.NotNil(t, a.Ip)
		assert.NotNil(t, a.Port)
	}
}

func TestLibrarian_Find_present(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)

	l := &Librarian{
		PeerID:    ecid.NewPseudoRandom(rng),
		db:        kvdb,
		serverSL:  storage.NewServerKVDBStorerLoader(kvdb),
		entriesSL: storage.NewEntriesKVDBStorerLoader(kvdb),
		kc:        storage.NewExactLengthChecker(storage.EntriesKeyLength),
	}

	// create key-value and store
	nValueBytes := 1014
	value := make([]byte, nValueBytes)
	nRead, err := rand.Read(value)
	assert.Equal(t, nValueBytes, nRead)
	assert.Nil(t, err)
	key := sha256.Sum256(value)

	err = l.entriesSL.Store(key[:], value)
	assert.Nil(t, err)

	// make request for key
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.FindRequest{
		Metadata: newTestRequestMetadata(rng, l.PeerID),
		Key:       key[:],
		NumPeers:  numClosest,
	}
	rp, err := l.Find(nil, rq)
	assert.Nil(t, err)

	// we should get back the value we stored
	assert.NotNil(t, rp.Value)
	assert.Nil(t, rp.Addresses)
	assert.True(t, bytes.Equal(value, rp.Value))
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Find_missing(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	n := 64
	rt, peerID := routing.NewTestWithPeers(rng, n)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)

	l := &Librarian{
		PeerID:    peerID,
		rt:        rt,
		db:        kvdb,
		serverSL:  storage.NewServerKVDBStorerLoader(kvdb),
		entriesSL: storage.NewEntriesKVDBStorerLoader(kvdb),
		kc:        storage.NewExactLengthChecker(storage.EntriesKeyLength),
	}

	// make request
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.FindRequest{
		Metadata: newTestRequestMetadata(rng, l.PeerID),
		Key:       cid.NewPseudoRandom(rng).Bytes(),
		NumPeers:  numClosest,
	}

	prevNumActivePeers := l.rt.NumPeers()
	rp, err := l.Find(nil, rq)
	assert.Nil(t, err)

	// should get peers since the value is missing
	checkPeersResponse(t, rq, rp, rt, prevNumActivePeers, numClosest)
}

func TestLibrarian_Store(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)

	l := &Librarian{
		PeerID:    ecid.NewPseudoRandom(rng),
		db:        kvdb,
		serverSL:  storage.NewServerKVDBStorerLoader(kvdb),
		entriesSL: storage.NewEntriesKVDBStorerLoader(kvdb),
		kc:        storage.NewExactLengthChecker(storage.EntriesKeyLength),
	}

	// create key-value
	key, value := newKeyValue(t, rng, 512)

	// make store request
	rq := &api.StoreRequest{
		Metadata: newTestRequestMetadata(rng, l.PeerID),
		Key:       key.Bytes(),
		Value:     value,
	}
	rp, err := l.Store(nil, rq)
	assert.Nil(t, err)
	assert.NotNil(t, rp)

	stored, err := l.entriesSL.Load(key.Bytes())
	assert.Nil(t, err)
	assert.True(t, bytes.Equal(value, stored))
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func newKeyValue(t *testing.T, rng *rand.Rand, nValueBytes int) (cid.ID, []byte) {
	value := make([]byte, nValueBytes)
	nRead, err := rng.Read(value)
	assert.Equal(t, nValueBytes, nRead)
	assert.Nil(t, err)
	key := sha256.Sum256(value)
	return cid.FromBytes(key[:]), value
}

func newTestRequestMetadata(rng *rand.Rand, peerID ecid.ID) *api.RequestMetadata {
	return &api.RequestMetadata{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		PubKey: ecid.ToPublicKeyBytes(peerID),
	}
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
	key, value := newKeyValue(t, rng, 512)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been found
	searchParams := search.NewParameters()
	foundValueResult := search.NewInitialResult(key, searchParams)
	foundValueResult.Value = value

	// create librarian and request
	l := newGetLibrarian(rng, foundValueResult, nil)
	rq := api.NewGetRequest(peerID, key)

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
	searchParams := search.NewParameters()
	foundClosestPeersResult := search.NewInitialResult(key, searchParams)
	dummyClosest := peer.NewTestPeers(rng, int(searchParams.NClosestResponses))
	err := foundClosestPeersResult.Closest.SafePushMany(dummyClosest)
	assert.Nil(t, err)

	// create librarian and request
	l := newGetLibrarian(rng, foundClosestPeersResult, nil)
	rq := api.NewGetRequest(peerID, key)

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
	searchParams := search.NewParameters()
	fatalErrorResult := search.NewInitialResult(key, searchParams)
	fatalErrorResult.FatalErr = errors.New("some fatal error")

	// create librarian and request
	l := newGetLibrarian(rng, fatalErrorResult, nil)
	rq := api.NewGetRequest(peerID, key)

	// since we have a fatal search error, Get() should also return an error
	rp, err := l.Get(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Get_Exhausted(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, peerID := cid.NewPseudoRandom(rng), ecid.NewPseudoRandom(rng)

	// create mock search result with Exhausted() true
	searchParams := search.NewParameters()
	exhaustedResult := search.NewInitialResult(key, searchParams)

	// create librarian and request
	l := newGetLibrarian(rng, exhaustedResult, nil)
	rq := api.NewGetRequest(peerID, key)

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
	rq := api.NewGetRequest(peerID, key)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Get(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func newGetLibrarian(rng *rand.Rand, searchResult *search.Result, searchErr error) *Librarian {
	n := 8
	rt, peerID := routing.NewTestWithPeers(rng, n)
	return &Librarian{
		PeerID: peerID,
		rt:     rt,
		kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
		searcher: &fixedSearcher{
			result: searchResult,
			err:    searchErr,
		},
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
	key, value := newKeyValue(t, rng, 512)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been stored
	searchParams := search.NewParameters()
	nReplicas := searchParams.NClosestResponses
	addedResult := store.NewInitialResult(search.NewInitialResult(key, searchParams))
	addedResult.Responded = peer.NewTestPeers(rng, int(nReplicas))

	// create librarian and request
	l := newPutLibrarian(rng, addedResult, nil)
	rq := api.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, uint32(nReplicas), rp.NReplicas)
	assert.Equal(t, api.PutOperation_STORED, rp.Operation)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Put_Exists(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, value := newKeyValue(t, rng, 512)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been stored
	searchParams := search.NewParameters()
	foundValueResult := search.NewInitialResult(key, searchParams)
	foundValueResult.Value = value
	existsResult := store.NewInitialResult(foundValueResult)

	// create librarian and request
	l := newPutLibrarian(rng, existsResult, nil)
	rq := api.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, api.PutOperation_LEFT_EXISTING, rp.Operation)
	assert.Equal(t, rq.Metadata.RequestId, rp.Metadata.RequestId)
}

func TestLibrarian_Put_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, value := newKeyValue(t, rng, 512)
	peerID := ecid.NewPseudoRandom(rng)

	// create mock search result where the value has been stored
	searchParams := search.NewParameters()
	erroredResult := store.NewInitialResult(search.NewInitialResult(key, searchParams))
	erroredResult.FatalErr = errors.New("some fatal error")

	// create librarian and request
	l := newPutLibrarian(rng, erroredResult, nil)
	rq := api.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestLibrarian_Put_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key, value := newKeyValue(t, rng, 512)
	peerID := ecid.NewPseudoRandom(rng)

	// create librarian and request
	l := newPutLibrarian(rng, nil, errors.New("some store error"))
	rq := api.NewPutRequest(peerID, key, value)

	// since fixedSearcher returns fixed value, should get that back in response
	rp, err := l.Put(nil, rq)
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func newPutLibrarian(rng *rand.Rand, storeResult *store.Result, searchErr error) *Librarian {
	n := 8
	rt, peerID := routing.NewTestWithPeers(rng, n)
	return &Librarian{
		PeerID: peerID,
		rt:     rt,
		kc:     storage.NewExactLengthChecker(storage.EntriesKeyLength),
		storer: &fixedStorer{
			result: storeResult,
			err:    searchErr,
		},
	}
}
