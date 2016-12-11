package server

import (
	"crypto/rand"

	"github.com/drausin/libri/librarian/api"
	"golang.org/x/net/context"
)

type librarian struct {
	// Config holds the configuration parameters of the server
	ServerConfig *Config

	// NodeID is the random 256-bit identification number of this node in the hash table
	NodeID []byte
}

func NewLibrarian() (*librarian, error) {
	nodeID, err := generateNodeID()
	if err != nil {
		return nil, err
	}

	return &librarian{
		ServerConfig: DefaultConfig(),
		NodeID:       nodeID,
	}, nil
}

// Generate a 256-bit random node ID
func generateNodeID() ([]byte, error) {
	nodeID := make([]byte, 32)
	_, err := rand.Read(nodeID)
	if err != nil {
		return nil, err
	}
	return nodeID, nil
}

func (l *librarian) Ping(ctx context.Context, rq *api.PingRequest) (*api.PingResponse, error) {
	return &api.PingResponse{Message: "pong"}, nil
}

func (l *librarian) Identify(ctx context.Context, rq *api.IdentityRequest) (*api.IdentityResponse, error) {
	return &api.IdentityResponse{
		NodeName: l.ServerConfig.NodeName,
		NodeId:   l.NodeID,
	}, nil
}
