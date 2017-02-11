package server

import (
	"fmt"
	"os"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/pkg/errors"
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

	// executes searches for peers and keys
	searcher search.Searcher

	// executes stores for key/value
	storer store.Storer

	// db is the key-value store DB used for all external storage
	db db.KVDB

	// SL for server data
	serverSL storage.NamespaceStorerLoader

	// SL for p2p stored records
	entriesSL storage.NamespaceStorerLoader

	// kc ensures keys are valid
	kc storage.Checker

	// rt is the routing table of peers
	rt routing.Table
}

// NewLibrarian creates a new librarian instance.
func NewLibrarian(config *Config) (*Librarian, error) {
	rdb, err := db.NewRocksDB(config.DbDir)
	if err != nil {
		return nil, err
	}
	serverSL := storage.NewServerKVDBStorerLoader(rdb)
	entriesSL := storage.NewEntriesKVDBStorerLoader(rdb)

	peerID, err := loadOrCreatePeerID(serverSL)
	if err != nil {
		return nil, err
	}
	if err = serverSL.Store(peerIDKey, peerID.Bytes()); err != nil {
		return nil, err
	}

	rt, err := loadOrCreateRoutingTable(serverSL, peerID)
	if err != nil {
		return nil, err
	}

	searcher := search.NewDefaultSearcher()

	return &Librarian{
		PeerID:    peerID,
		Config:    config,
		searcher:  searcher,
		storer:    store.NewStorer(searcher, store.NewQuerier()),
		db:        rdb,
		serverSL:  serverSL,
		entriesSL: entriesSL,
		kc:        storage.NewExactLengthChecker(storage.EntriesKeyLength),
		rt:        rt,
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

// Find returns either the value at a given target or the peers closest to it.
func (l *Librarian) Find(ctx context.Context, rq *api.FindRequest) (*api.FindResponse,
	error) {
	if err := l.kc.Check(rq.Key); err != nil {
		return nil, err
	}
	value, err := l.entriesSL.Load(rq.Key)
	if err != nil {
		// something went wrong during load
		return nil, err
	}

	// we have the value, so return it
	if value != nil {
		return &api.FindResponse{
			RequestId: rq.RequestId,
			Value:     value,
		}, nil
	}

	// otherwise, return the peers closest to the key
	key := cid.FromBytes(rq.Key)
	closest := l.rt.Peak(key, uint(rq.NumPeers))
	addresses := make([]*api.PeerAddress, len(closest))
	for i, peer := range closest {
		addresses[i] = peer.ToAPI()
	}
	return &api.FindResponse{
		RequestId: rq.RequestId,
		Addresses: addresses,
	}, nil
}

// Store stores the value
func (l *Librarian) Store(ctx context.Context, rq *api.StoreRequest) (
	*api.StoreResponse, error) {
	if err := l.entriesSL.Store(rq.Key, rq.Value); err != nil {
		return nil, err
	}
	return &api.StoreResponse{
		RequestId: rq.RequestId,
	}, nil
}

// Get returns the value for a given key, if it exists. This endpoint handles the internals of
// searching for the key.
func (l *Librarian) Get(ctx context.Context, rq *api.GetRequest) (*api.GetResponse, error) {
	if err := l.kc.Check(rq.Key); err != nil {
		return nil, err
	}
	key := cid.FromBytes(rq.Key)
	s := search.NewSearch(key, search.NewParameters())
	seeds := l.rt.Peak(key, s.Params.Concurrency)
	err := l.searcher.Search(s, seeds)
	if err != nil {
		return nil, err
	}
	if s.FoundValue() {
		// return the value found by the search
		return &api.GetResponse{
			RequestId: rq.RequestId,
			Value:     s.Result.Value,
		}, nil
	}
	if s.FoundClosestPeers() {
		// return the nil value, indicating that the value wasn't found
		return &api.GetResponse{
			RequestId: rq.RequestId,
			Value:     nil,
		}, nil
	}
	if s.Errored() {
		return nil, errors.New("search for key errored")
	}
	if s.Exhausted() {
		return nil, errors.New("search for key exhausted")
	}

	return nil, errors.New("unexpected search result")
}

// Put stores a given key and value. This endpoint handles the internals of finding the right
// peers to store the value in and then sending them store requests.
func (l *Librarian) Put(ctx context.Context, rq *api.PutRequest) (*api.PutResponse, error) {
	if err := l.kc.Check(rq.Key); err != nil {
		return nil, err
	}
	key := cid.FromBytes(rq.Key)
	s := store.NewStore(
		search.NewSearch(key, search.NewParameters()),
		rq.Value,
		store.NewParameters(),
	)
	seeds := l.rt.Peak(key, s.Search.Params.Concurrency)
	err := l.storer.Store(s, seeds)
	if err != nil {
		return nil, err
	}
	if s.Finished() {
		operation := api.PutOperation_ADDED
		if s.Exists() {
			operation = api.PutOperation_LEFT_EXISTING
		}
		return &api.PutResponse{
			RequestId: rq.RequestId,
			Operation: operation,
			NReplicas: uint32(len(s.Result.Responded)),
		}, nil
	}
	if s.Errored() {
		// TODO (drausin) better collect and surface errors from queries
		return nil, errors.New("received error during search or store operations")
	}

	return nil, errors.New("unexpected store result")
}
