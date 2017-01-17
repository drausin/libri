package server

import (
	"fmt"
	"os"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"golang.org/x/net/context"
)

var (
	peerIDKey = []byte("PeerID")
)

// Librarian is the main service of a single peer in the peer to peer network.
type Librarian struct {
	// PeerID is the random 256-bit identification number of this node in the hash table
	PeerID cid.ID

	// Config holds the configuration parameters of the server
	Config *Config

	// db is the key-value store DB used for all external storage
	db db.KVDB

	// rt is the routing table of peers
	rt routing.Table
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
		rt:     rt,
	}, nil
}

func loadOrCreatePeerID(db db.KVDB) (cid.ID, error) {
	peerIDB, err := db.Get(peerIDKey)
	if err != nil {
		return nil, err
	}

	if peerIDB != nil {
		// return saved PeerID
		return cid.FromBytes(peerIDB), nil
	}

	// create new PeerID
	peerID := cid.NewRandom()

	// TODO: move this up to a Librarian.Save() method
	// save new PeerID
	if db.Put(peerIDKey, peerID.Bytes()) != nil {
		return nil, err
	}
	return peerID, nil

}

func loadOrCreateRoutingTable(db db.KVDB, selfID cid.ID) (routing.Table, error) {
	rt, err := routing.Load(db)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		if selfID.Cmp(rt.SelfID()) != 0 {
			return nil, fmt.Errorf("selfID (%v) of loaded routing table does not "+
				"match Librarian selfID (%v)", rt.SelfID(), selfID)
		}
		return rt, nil
	}

	return routing.NewEmpty(selfID), nil
}

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {
	if err := l.rt.Disconnect(); err != nil {
		return err
	}
	if err := l.rt.Save(l.db); err != nil {
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
		PeerName:  l.Config.PeerName,
		PeerId:    l.PeerID.Bytes(),
	}, nil
}

// FindPeers returns the closest peers to a given target.
func (l *Librarian) FindPeers(ctx context.Context, rq *api.FindRequest) (*api.FindPeersResponse,
	error) {
	target := cid.FromBytes(rq.Target)
	closest := l.rt.Peak(target, uint(rq.NumPeers))
	addresses := make([]*api.PeerAddress, len(closest))
	for i, peer := range closest {
		addresses[i] = api.FromAddress(peer.ID(), peer.PublicAddress())
	}
	return &api.FindPeersResponse{
		RequestId: rq.RequestId,
		Addresses: addresses,
	}, nil
}
