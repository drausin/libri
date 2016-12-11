package server

import (
	"testing"
	"github.com/drausin/libri/librarian/api"
	"encoding/base64"
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
		ServerConfig: &Config{
			NodeName: nodeName,
		},
		NodeID: nodeId,
	}

	r, err := lib.Identify(nil, &api.IdentityRequest{})
	assert.Nil(t, err)
	assert.Equal(t, r.NodeId, nodeId)
	assert.Equal(t, r.NodeName, nodeName)
}

