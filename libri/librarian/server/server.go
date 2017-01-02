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
	// IDLength is the number of bytes in an ID
	IDLength = 32
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

	// db is the key-value store DB used for all external storage
	db db.KVDB

	// rt is the routing table of peers
	rt *RoutingTable
}

// NewLibrarian creates a new librarian instance.
func NewLibrarian(config *Config) (*Librarian, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		return nil, err
	}

	peerID, err := loadOrCreatePeerID(rdb)
	if err != nil {
		return nil, err
	}

	rt, err := loadOrCreateRoutingTable(rdb, peerID)
	if err != nil {
		return nil, err
	}

	return &Librarian{
		PeerID: peerID,
		Config: config,
		db:     rdb,
		rt: 	rt,
	}, nil
}

func loadOrCreatePeerID(db db.KVDB) (*big.Int, error) {
	peerIDB, err := db.Get(peerIDKey)
	if err != nil {
		return nil, err
	}

	if peerIDB != nil {
		// return saved PeerID
		return NewID(peerIDB), nil
	}

	// create new PeerID
	peerID, err := NewRandomID()
	if err != nil {
		return nil, err
	}

	// save new PeerID
	if db.Put(peerIDKey, peerID.Bytes()) != nil {
		return nil, err
	}
	return peerID, nil

}

func loadOrCreateRoutingTable(db db.KVDB, selfID *big.Int) (*RoutingTable, error) {
	rt, err := LoadRoutingTable(db)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		return rt, nil
	}

	return NewEmptyRoutingTable(selfID), nil
}

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {
	if err := l.rt.Close(l.db); err != nil {
		return err
	}
	l.db.Close()

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
		RequestId: rq.RequestId,
		PeerName: l.Config.PeerName,
		PeerId:   l.PeerID.Bytes(),
	}, nil
}

// FindPeers returns the closest peers to a given target.
func (l *Librarian) FindPeers(ctx context.Context, rq *api.FindRequest) (*api.FindPeersResponse,
	error) {
	target := NewID(rq.Target)
	closest, _, err := l.rt.PeakNextPeers(target, int(rq.NumPeers))
	if err != nil {
		return nil, err
	}
	foundPeers := make([]*api.Peer, len(closest))
	for i, peer := range closest {
		foundPeers[i] = peer.NewAPIPeer()
	}
	return &api.FindPeersResponse{
		RequestId: rq.RequestId,
		Peers:     foundPeers,
	}, nil
}

// NewRandomID returns a random 32-byte ID.
func NewRandomID() (*big.Int, error) {
	idB := make([]byte, IDLength)
	_, err := rand.Read(idB)
	if err != nil {
		return nil, err
	}
	return NewID(idB), nil
}

// NewID creates a *big.Int from a big-endian byte array.
func NewID(id []byte) *big.Int {
	return big.NewInt(0).SetBytes(id)
}
