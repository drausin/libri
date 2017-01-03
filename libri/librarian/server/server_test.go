package server

import (
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common"
	"github.com/drausin/libri/libri/librarian/api"
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
	peerID := newPseudoRandomID(rng)
	peerName := "Test Node"
	lib := &Librarian{
		Config: &Config{
			PeerName: peerName,
		},
		PeerID: peerID,
	}

	requestID := newPseudoRandomID(rng)
	rq := &api.IdentityRequest{RequestId: requestID.Bytes()}
	rp, err := lib.Identify(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, rq.RequestId, rp.RequestId)
	assert.Equal(t, peerID.Bytes(), rp.PeerId)
	assert.Equal(t, peerName, rp.PeerName)
}

func TestLibrarian_FindPeers(t *testing.T) {
	for nPeers := 8; nPeers <= 128; nPeers *= 2 {
		// for different numbers of total active peers

		for s := 0; s < 10; s++ {
			// for different selfIDs

			rng := rand.New(rand.NewSource(int64(s)))
			selfID := newPseudoRandomID(rng)
			rt, _, err := NewRoutingTableWithPeers(selfID, generatePeers(nPeers, rng))
			assert.Nil(t, err)
			l := &Librarian{
				PeerID: selfID,
				rt:     rt,
			}

			numClosest := uint32(defaultMaxActivePeers)
			rq := &api.FindRequest{
				RequestId: newPseudoRandomID(rng).Bytes(),
				Target:    newPseudoRandomID(rng).Bytes(),
				NumPeers:  numClosest,
			}

			beforeNumActivePeers := l.rt.NumActivePeers()
			rp, err := l.FindPeers(nil, rq)

			// check
			assert.Nil(t, err)
			assert.Equal(t, rq.RequestId, rp.RequestId)
			assert.Equal(t, beforeNumActivePeers, l.rt.NumActivePeers())
			if int(numClosest) > beforeNumActivePeers {
				assert.Equal(t, beforeNumActivePeers, len(rp.Peers))
			} else {
				assert.Equal(t, numClosest, uint32(len(rp.Peers)))
			}
			for _, peer := range rp.Peers {
				assert.NotNil(t, peer.PeerId)
				assert.NotNil(t, peer.AddressIp)
				assert.NotNil(t, peer.AddressPort)
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
	defer common.MaybePanic(lib2.Close())
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
