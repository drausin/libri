package server

import (
	"io/ioutil"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/stretchr/testify/assert"
	"bytes"
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
	peerID := cid.NewPseudoRandom(rng)
	peerName := "Test Node"
	lib := &Librarian{
		Config: &Config{
			PeerName: peerName,
		},
		PeerID: peerID,
	}

	requestID := cid.NewPseudoRandom(rng)
	rq := &api.IdentityRequest{RequestId: requestID.Bytes()}
	rp, err := lib.Identify(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, rq.RequestId, rp.RequestId)
	assert.Equal(t, peerID.Bytes(), rp.PeerId)
	assert.Equal(t, peerName, rp.PeerName)
}

func TestLibrarian_FindPeers(t *testing.T) {
	for n := 8; n <= 128; n *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs

			rng := rand.New(rand.NewSource(int64(s)))
			rt := routing.NewTestWithPeers(rng, n)
			l := &Librarian{
				PeerID: rt.SelfID(),
				rt:     rt,
			}

			numClosest := uint32(routing.DefaultMaxActivePeers)
			rq := &api.FindRequest{
				RequestId: cid.NewPseudoRandom(rng).Bytes(),
				Target:    cid.NewPseudoRandom(rng).Bytes(),
				NumPeers:  numClosest,
			}

			prevNumActivePeers := l.rt.NumPeers()
			rp, err := l.FindPeers(nil, rq)
			assert.Nil(t, err)

			// check
			checkPeersResponse(t, rq, rp, rt, prevNumActivePeers, numClosest)
		}
	}
}

func checkPeersResponse(t *testing.T, rq *api.FindRequest, rp *api.FindResponse, rt routing.Table,
	prevNumActivePeers uint, numClosest uint32) {

	assert.Equal(t, rq.RequestId, rp.RequestId)
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

func TestLibrarian_FindValue_present(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	sl := storage.NewKVDBStorerLoader(kvdb)

	l := &Librarian{
		PeerID:    cid.NewPseudoRandom(rng),
		db:        kvdb,
		serverSL:  storage.NewServerStorerLoader(sl),
		recordsSL: storage.NewServerStorerLoader(sl),
	}

	// create key-value and store
	key := cid.NewPseudoRandom(rng).Bytes()
	nValueBytes := 1014
	value := make([]byte, nValueBytes)
	nRead, err := rand.Read(value)
	assert.Equal(t, nValueBytes, nRead)
	assert.Nil(t, err)
	err = l.recordsSL.Store(key, value)
	assert.Nil(t, err)

	// make request for key
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.FindRequest{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Target:    key,
		NumPeers:  numClosest,
	}
	rp, err := l.FindValue(nil, rq)
	assert.Nil(t, err)

	// we should get back the value we stored
	assert.NotNil(t, rp.Value)
	assert.Nil(t, rp.Addresses)
	assert.True(t, bytes.Equal(value, rp.Value))
}

func TestLibrarian_FindValue_missing(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	n := 64
	rt := routing.NewTestWithPeers(rng, n)
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	sl := storage.NewKVDBStorerLoader(kvdb)

	l := &Librarian{
		PeerID:    rt.SelfID(),
		rt:        rt,
		db:        kvdb,
		serverSL:  storage.NewServerStorerLoader(sl),
		recordsSL: storage.NewServerStorerLoader(sl),
	}

	// make request
	numClosest := uint32(routing.DefaultMaxActivePeers)
	rq := &api.FindRequest{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Target:    cid.NewPseudoRandom(rng).Bytes(),
		NumPeers:  numClosest,
	}

	prevNumActivePeers := l.rt.NumPeers()
	rp, err := l.FindValue(nil, rq)
	assert.Nil(t, err)

	// should get peers since the value is missing
	checkPeersResponse(t, rq, rp, rt, prevNumActivePeers, numClosest)
}

func TestLibrarian_Store(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	kvdb, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	sl := storage.NewKVDBStorerLoader(kvdb)

	l := &Librarian{
		PeerID:    cid.NewPseudoRandom(rng),
		db:        kvdb,
		serverSL:  storage.NewServerStorerLoader(sl),
		recordsSL: storage.NewServerStorerLoader(sl),
	}

	// create key-value
	key := cid.NewPseudoRandom(rng).Bytes()
	nValueBytes := 1014
	value := make([]byte, nValueBytes)
	nRead, err := rand.Read(value)
	assert.Equal(t, nValueBytes, nRead)
	assert.Nil(t, err)

	// make store request
	rq := &api.StoreRequest{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Key: key,
		Value: value,
	}
	rp, err := l.Store(nil, rq)
	assert.Equal(t, api.StoreStatus_SUCCEEDED, rp.Status)

	stored, err := l.recordsSL.Load(key)
	assert.Nil(t, err)
	assert.True(t, bytes.Equal(value, stored))
}