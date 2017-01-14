package server

import (
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/stretchr/testify/assert"
)

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
	peerID := id.NewPseudoRandom(rng)
	peerName := "Test Node"
	lib := &Librarian{
		Config: &Config{
			PeerName: peerName,
		},
		PeerID: peerID,
	}

	requestID := id.NewPseudoRandom(rng)
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
			selfID := id.NewPseudoRandom(rng)
			rt := routing.NewTestWithPeers(rng, n)
			l := &Librarian{
				PeerID: selfID,
				rt:     rt,
			}

			numClosest := uint32(routing.DefaultMaxActivePeers)
			rq := &api.FindRequest{
				RequestId: id.NewPseudoRandom(rng).Bytes(),
				Target:    id.NewPseudoRandom(rng).Bytes(),
				NumPeers:  numClosest,
			}

			beforeNumActivePeers := l.rt.NumPeers()
			rp, err := l.FindPeers(nil, rq)
			assert.Nil(t, err)

			// check
			assert.Equal(t, rq.RequestId, rp.RequestId)
			assert.Equal(t, beforeNumActivePeers, l.rt.NumPeers())
			if uint(numClosest) > beforeNumActivePeers {
				assert.Equal(t, int(beforeNumActivePeers), len(rp.Addresses))
			} else {
				assert.Equal(t, numClosest, uint32(len(rp.Addresses)))
			}
			for _, a := range rp.Addresses {
				assert.NotNil(t, a.PeerId)
				assert.NotNil(t, a.Ip)
				assert.NotNil(t, a.Port)
			}
		}
	}
}

// TestNewLibrarian checks that we can create a new instance, close it, and create it again as
// expected.
func TestNewLibrarian(t *testing.T) {
	lib1 := newTestLibrarian()

	nodeID1 := lib1.PeerID // should have been generated
	err := lib1.Close()
	assert.Nil(t, err)

	lib2, err := NewLibrarian(lib1.Config)
	assert.Nil(t, err)
	lib2.Close()
	assert.Equal(t, nodeID1, lib2.PeerID)
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
