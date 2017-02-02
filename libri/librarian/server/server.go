package server

import (
	"fmt"
	"os"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"golang.org/x/net/context"
	"github.com/pkg/errors"
)

var (
	peerIDKey = []byte("PeerID")
)

// Librarian is the main service of a single peer in the peer to peer network.
type Librarian struct {
	// PeerID is the random 256-bit identification number of this node in the hash table
	PeerID    cid.ID

	// Config holds the configuration parameters of the server
	Config    *Config

	// db is the key-value store DB used for all external storage
	db        db.KVDB

	// SL for server data
	serverSL  storage.NamespaceStorerLoader

	// SL for p2p stored records
	recordsSL storage.NamespaceStorerLoader

	// rt is the routing table of peers
	rt        routing.Table
}

// NewLibrarian creates a new librarian instance.
func NewLibrarian(config *Config) (*Librarian, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		return nil, err
	}
	sl := storage.NewKVDBStorerLoader(rdb)
	serverSL := storage.NewServerStorerLoader(sl)
	recordsSL := storage.NewRecordsStorerLoader(sl)

	peerID, err := loadOrCreatePeerID(serverSL)
	if err != nil {
		return nil, err
	}
	if err := serverSL.Store(peerIDKey, peerID.Bytes()); err != nil {
		return nil, err
	}

	rt, err := loadOrCreateRoutingTable(serverSL, peerID)
	if err != nil {
		return nil, err
	}

	return &Librarian{
		PeerID:   peerID,
		Config:   config,
		db:       rdb,
		serverSL: serverSL,
		recordsSL:  recordsSL,
		rt:       rt,
	}, nil
}

func loadOrCreatePeerID(nl storage.NamespaceLoader) (cid.ID, error) {
	peerIDB, err := nl.Load(peerIDKey)
	if err != nil {
		return nil, err
	}

	if peerIDB != nil {
		// return saved PeerID
		return cid.FromBytes(peerIDB), nil
	}

	// return new PeerID
	return cid.NewRandom(), nil
}

func loadOrCreateRoutingTable(nl storage.NamespaceLoader, selfID cid.ID) (routing.Table, error) {
	rt, err := routing.Load(nl)
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
	if err := l.rt.Save(l.serverSL); err != nil {
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
func (l *Librarian) FindPeers(ctx context.Context, rq *api.FindRequest) (*api.FindResponse,
	error) {
	if err := checkTarget(rq.Target); err != nil {
		return nil, err
	}
	target := cid.FromBytes(rq.Target)
	closest := l.rt.Peak(target, uint(rq.NumPeers))
	addresses := make([]*api.PeerAddress, len(closest))
	for i, peer := range closest {
		addresses[i] = peer.ToAPI()
	}
	return &api.FindResponse{
		RequestId: rq.RequestId,
		Addresses: addresses,
	}, nil
}

func (l *Librarian) FindValue(ctx context.Context, rq *api.FindRequest) (*api.FindResponse,
	error) {
	if err := checkTarget(rq.Target); err != nil {
		return nil, err
	}
	value, err := l.recordsSL.Load(rq.Target)
	if err != nil {
		// something went wrong during load
		return nil, err
	}
	if value != nil {
		// we have the value, so return it
		return &api.FindResponse{
			RequestId: rq.RequestId,
			Value: value,
		}, nil
	}

	// otherwise, return peers closest to the target
	return l.FindPeers(ctx, rq)
}

func (l *Librarian) Store(ctx context.Context, rq *api.StoreRequest) (
	*api.StoreResponse, error) {
	if err := checkTarget(rq.Key); err != nil {
		return nil, err
	}
	if err := l.recordsSL.Store(rq.Key, rq.Value); err != nil {
		return nil, err
	}
	return &api.StoreResponse{
		RequestId: rq.RequestId,
		Status: api.StoreStatus_SUCCEEDED,
	}, nil
}

func checkTarget(target []byte) error {
	if target == nil || len(target) == 0 {
		return errors.New("request target not specified")
	}
	if len(target) > cid.Length {
		return fmt.Errorf("request target has length %v > %v", len(target), cid.Length)
	}
	return nil
}
