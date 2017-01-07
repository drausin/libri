package storage

import (
	"net"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/common/id"
	"time"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/db"
	"github.com/gogo/protobuf/proto"
)

const (
	// Keys are the []byte keys used for storing different components.
	Keys = struct{
		RoutingTable []byte
	}{
		RoutingTable: []byte("RoutingTable"),
	}
)


// ToAddress creates a net.TCPAddr from a storage.Address.
func ToAddress(stored *Address) *net.TCPAddr {
	return &net.TCPAddr{
		IP: net.ParseIP(stored.Ip),
		Port: int(stored.Port),
	}
}

// FromAddress creates a storage.Address from a net.TCPAddr.
func FromAddress(address *net.TCPAddr) *Address {
	return Address{
		Ip: address.IP.String(),
		Port: uint32(address.Port),
	}
}

// ToResponseStats creates a peer.ResponseStats from a storage.ResponseStats
func ToResponseStats(stored *ResponseStats) *peer.ResponseStats {
	return &peer.ResponseStats{
		Earliest: time.Unix(stored.Earliest, int64(0)).UTC(),
		Latest: time.Unix(stored.Earliest, int64(0)).UTC(),
		NQueries: stored.NQueries,
		NErrors: stored.NErrors,
	}
}

// FromResponseStats creates a storage.ResponseStats from a peer.ResponseStats.
func FromResponseStats(stats *peer.ResponseStats) *ResponseStats {
	return ResponseStats{
		Earliest: stats.Earliest.Unix(),
		Latest: stats.Latest.Unix(),
		NQueries: stats.NQueries,
		NErrors: stats.NErrors,
	}
}

// ToPeer creates a new server.Peer instance from a storage.Peer instance.
func ToPeer(stored *Peer) *peer.Peer {
	return peer.New(
		id.FromBytes(stored.Id),
		stored.Name,
		ToAddress(stored.PublicAddress),
	)
}

// FromPeer creates a storage.Peer from a peer.Peer.
func FromPeer(peer *peer.Peer) *Peer {
	return Peer{
		Id: peer.ID.Bytes(),
		Name: peer.Name,
		PublicAddress: FromAddress(peer.PublicAddress),
		Responses: FromResponseStats(peer.Responses),
	}
}

// NewRoutingTableFromStorage returns a new RoutingTable instance from a
// StoredRoutingTable instance.
func ToRoutingTable(stored *RoutingTable) (*RoutingTable, error) {
	peers := make([]*peer.Peer, len(stored.Peers))
	for i, sp := range stored.Peers {
		peers[i] = ToPeer(sp)
	}
	return routing.NewWithPeers(id.FromBytes(stored.SelfId), peers)
}

// NewStoredRoutingTable creates a new StoredRoutingTable instance from the RoutingTable instance.
func FromRoutingTable(rt *routing.RoutingTable) *RoutingTable {
	storedPeers := make([]*Peer, len(rt.Peers))
	i := 0
	for _, p := range rt.Peers {
		storedPeers[i] = FromPeer(p)
		i++
	}
	return &RoutingTable{
		SelfId: rt.SelfID.Bytes(),
		Peers:  storedPeers,
	}
}

// Load retrieves the routing table form the KV DB.
func LoadRoutingTable(db db.KVDB) (*RoutingTable, error) {
	bytes, err := db.Get(Keys.RoutingTable)
	if bytes == nil || err != nil {
		return nil, err
	}
	stored := &RoutingTable{}
	err = proto.Unmarshal(bytes, stored)
	if err != nil {
		return nil, err
	}
	return ToRoutingTable(stored)
}

// Save stores a representation of the routing table to the KV DB.
func SaveRoutingTable(db db.KVDB, rt *routing.RoutingTable) error {
	bytes, err := proto.Marshal(FromRoutingTable(rt))
	if err != nil {
		return err
	}
	return db.Put(Keys.RoutingTable, bytes)
}

