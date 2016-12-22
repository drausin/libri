package server

import (
	"encoding/base64"
	"io/ioutil"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

// TestLibrarian_Ping verifies that we receive the expected response ("pong") to a ping request.
func TestLibrarian_Ping(t *testing.T) {
	lib := &librarian{}
	r, err := lib.Ping(nil, &api.PingRequest{})
	assert.Nil(t, err)
	assert.Equal(t, r.Message, "pong")
}

// TestLibrarian_Identify verifies that we get the expected response from a an identification request.
func TestLibrarian_Identify(t *testing.T) {
	nodeId, err := base64.URLEncoding.DecodeString("JdZw91-N6uNMBvQ3-tx9CQVP3D870C9mnvfhLD9C6yU=")
	assert.Nil(t, err)
	nodeName := "Test Node"
	lib := &librarian{
		Config: &Config{
			NodeName: nodeName,
		},
		NodeID: nodeId,
	}

	r, err := lib.Identify(nil, &api.IdentityRequest{})
	assert.Nil(t, err)
	assert.Equal(t, r.NodeId, nodeId)
	assert.Equal(t, r.NodeName, nodeName)
}

// TestNewLibrarian checks that we can create a new instance, close it, and create it again as expected.
func TestNewLibrarian(t *testing.T) {
	config := DefaultConfig()
	dir, err := ioutil.TempDir("", "test-data-dir")
	assert.Nil(t, err)
	config.SetDataDir(dir)

	lib1, err := NewLibrarian(config)
	assert.Nil(t, err)
	nodeID1 := lib1.NodeID // should have been generated
	lib1.Close()

	lib2, err := NewLibrarian(config)
	assert.Nil(t, err)
	defer lib2.Close()
	assert.Equal(t, nodeID1, lib2.NodeID)
}
