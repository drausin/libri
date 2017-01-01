package server

import (
	"crypto/rand"
	"math/big"
	"os"

	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
)

const (
	// PeerIDLength is the number of bytes in a peer ID.
	PeerIDLength = 32
)

var (
	peerIDKey = []byte("PeerID")
)

// Librarian is the main service of a single peer in the peer to peer network.
type Librarian struct {
	// PeerID is the random 256-bit identification number of this node in the hash table
	PeerID *big.Int

	// Config holds the configuration parameters of the server
	Config *Config

	// DB is the key-value store DB used for all external storage
	DB db.KVDB
}

// NewLibrarian creates a new librarian instance.
func NewLibrarian(config *Config) (*Librarian, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		return nil, err
	}

	var peerID *big.Int
	peerIDB, err := rdb.Get(peerIDKey)
	if err != nil {
		return nil, err
	}
	if peerIDB != nil {
		peerID = big.NewInt(0).SetBytes(peerIDB)
	} else {
		peerID, err = generatePeerID()
		if err != nil {
			return nil, err
		}
		if rdb.Put(peerIDKey, peerID.Bytes()) != nil {
			return nil, err
		}
	}

	return &Librarian{
		PeerID: peerID,
		Config: config,
		DB:     rdb,
	}, nil
}

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {
	l.DB.Close()
	return nil
}

// CloseAndRemove cleans up and removes any local state from the server.
func (l *Librarian) CloseAndRemove() error {
	err := l.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(l.Config.DataDir)
}

// Ping confirms simple request/response connectivity.
func (l *Librarian) Ping(ctx context.Context, rq *api.PingRequest) (*api.PingResponse, error) {
	return &api.PingResponse{Message: "pong"}, nil
}

// Identify gives the identifying information about the peer in the network.
func (l *Librarian) Identify(ctx context.Context, rq *api.IdentityRequest) (*api.IdentityResponse,
	error) {
	return &api.IdentityResponse{
		PeerName: l.Config.PeerName,
		PeerId:   l.PeerID.Bytes(),
	}, nil
}

// generatePeerID returns a random peer ID.
func generatePeerID() (*big.Int, error) {
	idB := make([]byte, PeerIDLength)
	_, err := rand.Read(idB)
	if err != nil {
		return nil, err
	}
	return big.NewInt(0).SetBytes(idB), nil
}
