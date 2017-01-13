package routing

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/db"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/golang/protobuf/proto"
)

var tableKey = []byte("RoutingTable")

// LoadRoutingTable retrieves the routing table form the KV DB.
func Load(db db.KVDB) (*Table, error) {
	bytes, err := db.Get(tableKey)
	if bytes == nil || err != nil {
		return nil, err
	}
	stored := &storage.RoutingTable{}
	err = proto.Unmarshal(bytes, stored)
	if err != nil {
		return nil, err
	}
	return fromStored(stored), nil
}

// SaveRoutingTable stores a representation of the routing table to the KV DB.
func Save(db db.KVDB, rt *Table) error {
	bytes, err := proto.Marshal(toStored(rt))
	if err != nil {
		return err
	}
	return db.Put(tableKey, bytes)
}

// ToRoutingTable returns a new RoutingTable instance from a
// StoredRoutingTable instance.
func fromStored(stored *storage.RoutingTable) *Table {
	peers := make([]*peer.Peer, len(stored.Peers))
	for i, sp := range stored.Peers {
		peers[i] = peer.FromStored(sp)
	}
	return NewWithPeers(id.FromBytes(stored.SelfId), peers)
}

// FromRoutingTable creates a new StoredRoutingTable instance from the RoutingTable instance.
func toStored(rt *Table) *storage.RoutingTable {
	storedPeers := make([]*storage.Peer, len(rt.Peers))
	i := 0
	for _, p := range rt.Peers {
		storedPeers[i] = peer.ToStored(p)
		i++
	}
	return &storage.RoutingTable{
		SelfId: rt.SelfID.Bytes(),
		Peers:  storedPeers,
	}
}
