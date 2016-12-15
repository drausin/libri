package server

import (
	"crypto/rand"

	"github.com/drausin/libri/db"
	"github.com/drausin/libri/librarian/api"
	"golang.org/x/net/context"
	"os"
)

const (
	NodeIDLength = 32
)

var (
	NodeIDKey = []byte("NodeID")
)

type librarian struct {
	// NodeID is the random 256-bit identification number of this node in the hash table
	NodeID []byte

	// Config holds the configuration parameters of the server
	Config *Config

	// DB is the key-value store DB used for all external storage
	DB     db.KVDB
}

// NewLibrarian creates a new librarian instance.
func NewLibrarian(config *Config) (*librarian, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		return nil, err
	}

	nodeID, err := rdb.Get(NodeIDKey)
	if err != nil {
		return nil, err
	}
	if nodeID == nil {
		nodeID, err = generateNodeID()
		if err != nil {
			return nil, err
		}
		if rdb.Put(NodeIDKey, nodeID) != nil {
			return nil, err
		}
	}

	return &librarian{
		NodeID: nodeID,
		Config: config,
		DB:     rdb,
	}, nil
}

// Close handles cleanup involved in closing down the server.
func (l *librarian) Close() error {
	return l.DB.Close()
}

// CloseAndRemove cleans up and removes any local state from the server.
func (l *librarian) CloseAndRemove() error {
	err := l.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(l.Config.DataDir)
}

func (l *librarian) Ping(ctx context.Context, rq *api.PingRequest) (*api.PingResponse, error) {
	return &api.PingResponse{Message: "pong"}, nil
}

func (l *librarian) Identify(ctx context.Context, rq *api.IdentityRequest) (*api.IdentityResponse, error) {

	return &api.IdentityResponse{
		NodeName: l.Config.NodeName,
		NodeId:   l.NodeID,
	}, nil
}

// Generate a random node ID
func generateNodeID() ([]byte, error) {
	nodeID := make([]byte, NodeIDLength)
	_, err := rand.Read(nodeID)
	if err != nil {
		return nil, err
	}
	return nodeID, nil
}
