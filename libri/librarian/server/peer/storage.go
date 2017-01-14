package peer

import (
	"net"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

// FromStored creates a new peer.Peer instance from a storage.Peer instance.
func FromStored(stored *storage.Peer) Peer {
	return NewWithResponseStats(
		id.FromBytes(stored.Id),
		stored.Name,
		fromStoredAddress(stored.PublicAddress),
		fromStoredResponseStats(stored.Responses),
	)
}

// ToStored creates a storage.Peer from a peer.Peer.
func (p *peer) ToStored() *storage.Peer {
	return &storage.Peer{
		Id:            p.id.Bytes(),
		Name:          p.name,
		PublicAddress: toStoredAddress(p.publicAddress),
		Responses:     p.responses.toStoredResponseStats(),
	}
}

// fromStoredAddress creates a net.TCPAddr from a storage.Address.
func fromStoredAddress(stored *storage.Address) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   net.ParseIP(stored.Ip),
		Port: int(stored.Port),
	}
}

// toStoredAddress creates a storage.Address from a net.TCPAddr.
func toStoredAddress(address *net.TCPAddr) *storage.Address {
	return &storage.Address{
		Ip:   address.IP.String(),
		Port: uint32(address.Port),
	}
}

// fromStoredResponseStats creates a peer.ResponseStats from a storage.ResponseStats
func fromStoredResponseStats(stored *storage.ResponseStats) *responseStats {
	return &responseStats{
		earliest: time.Unix(stored.Earliest, int64(0)).UTC(),
		latest:   time.Unix(stored.Earliest, int64(0)).UTC(),
		nQueries: stored.NQueries,
		nErrors:  stored.NErrors,
	}
}

// toStoredResponseStats creates a storage.ResponseStats from a peer.ResponseStats.
func (rs *responseStats) toStoredResponseStats() *storage.ResponseStats {
	return &storage.ResponseStats{
		Earliest: rs.earliest.Unix(),
		Latest:   rs.latest.Unix(),
		NQueries: rs.nQueries,
		NErrors:  rs.nErrors,
	}
}
