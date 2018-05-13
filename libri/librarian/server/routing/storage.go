package routing

import (
	"github.com/drausin/libri/libri/common/id"
	cstorage "github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/peer"
	sstorage "github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/golang/protobuf/proto"
)

var tableKey = []byte("RoutingTable")

// Load retrieves the routing table form the KV DB.
func Load(
	nl cstorage.Loader, preferer comm.Preferer, doctor comm.Doctor, params *Parameters,
) (Table, error) {
	bytes, err := nl.Load(tableKey)
	if bytes == nil || err != nil {
		return nil, err
	}
	stored := &sstorage.RoutingTable{}
	err = proto.Unmarshal(bytes, stored)
	if err != nil {
		return nil, err
	}
	return fromStored(stored, params, preferer, doctor), nil
}

// Save stores a representation of the routing table to the KV DB.
func (rt *table) Save(ns cstorage.Storer) error {
	bytes, err := proto.Marshal(toStored(rt))
	if err != nil {
		return err
	}
	return ns.Store(tableKey, bytes)
}

// fromStored returns a new Table instance from a StoredRoutingTable instance.
func fromStored(
	stored *sstorage.RoutingTable,
	params *Parameters,
	preferer comm.Preferer,
	doctor comm.Doctor,
) Table {
	peers := make([]peer.Peer, len(stored.Peers))
	for i, sp := range stored.Peers {
		peers[i] = peer.FromStored(sp)
	}
	rt, _ := NewWithPeers(id.FromBytes(stored.SelfId), preferer, doctor, params, peers)
	return rt
}

// toStored creates a new StoredRoutingTable instance from the Table instance.
func toStored(rt Table) *sstorage.RoutingTable {
	storedPeers := make([]*sstorage.Peer, len(rt.(*table).peers))
	i := 0
	for _, p := range rt.(*table).peers {
		storedPeers[i] = p.ToStored()
		i++
	}
	return &sstorage.RoutingTable{
		SelfId: rt.SelfID().Bytes(),
		Peers:  storedPeers,
	}
}
