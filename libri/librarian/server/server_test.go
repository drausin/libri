package server

import (
	"io/ioutil"
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
	peerID, err := NewRandomID()
	assert.Nil(t, err)
	peerName := "Test Node"
	lib := &Librarian{
		Config: &Config{
			PeerName: peerName,
		},
		PeerID: peerID,
	}

	requestID, err := NewRandomID()
	assert.Nil(t, err)
	rq := &api.IdentityRequest{RequestId: requestID.Bytes()}
	rp, err := lib.Identify(nil, rq)
	assert.Nil(t, err)
	assert.Equal(t, peerID.Bytes(), rp.PeerId)
	assert.Equal(t, peerName, rp.PeerName)
	assert.Equal(t, rq.RequestId, rp.RequestId)
}

// TestNewLibrarian checks that we can create a new instance, close it, and create it again as
// expected.
func TestNewLibrarian(t *testing.T) {
	config := DefaultConfig()
	dir, err := ioutil.TempDir("", "test-data-dir")
	assert.Nil(t, err)
	config.SetDataDir(dir)

	lib1, err := NewLibrarian(config)
	assert.Nil(t, err)
	nodeID1 := lib1.PeerID // should have been generated
	err = lib1.Close()
	assert.Nil(t, err)

	lib2, err := NewLibrarian(config)
	assert.Nil(t, err)
	defer common.MaybePanic(lib2.Close())
	assert.Equal(t, nodeID1, lib2.PeerID)
}
