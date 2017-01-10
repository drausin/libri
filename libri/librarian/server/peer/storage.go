package peer

import (
	"net"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

// ToPeer creates a new server.Peer instance from a storage.Peer instance.
func FromStored(stored *storage.Peer) *Peer {
	to := New(
		id.FromBytes(stored.Id),
		stored.Name,
		fromStoredAddress(stored.PublicAddress),
	)
	to.Responses = fromStoredResponseStats(stored.Responses)
	return to
}

// FromPeer creates a storage.Peer from a peer.Peer.
func ToStored(peer *Peer) *storage.Peer {
	return &storage.Peer{
		Id:            peer.ID.Bytes(),
		Name:          peer.Name,
		PublicAddress: toStoredAddress(peer.PublicAddress),
		Responses:     toStoredResponseStats(peer.Responses),
	}
}

// ToAddress creates a net.TCPAddr from a storage.Address.
func fromStoredAddress(stored *storage.Address) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   net.ParseIP(stored.Ip),
		Port: int(stored.Port),
	}
}

// FromAddress creates a storage.Address from a net.TCPAddr.
func toStoredAddress(address *net.TCPAddr) *storage.Address {
	return &storage.Address{
		Ip:   address.IP.String(),
		Port: uint32(address.Port),
	}
}

// ToResponseStats creates a peer.ResponseStats from a storage.ResponseStats
func fromStoredResponseStats(stored *storage.ResponseStats) *ResponseStats {
	return &ResponseStats{
		Earliest: time.Unix(stored.Earliest, int64(0)).UTC(),
		Latest:   time.Unix(stored.Earliest, int64(0)).UTC(),
		NQueries: stored.NQueries,
		NErrors:  stored.NErrors,
	}
}

// FromResponseStats creates a storage.ResponseStats from a peer.ResponseStats.
func toStoredResponseStats(stats *ResponseStats) *storage.ResponseStats {
	return &storage.ResponseStats{
		Earliest: stats.Earliest.Unix(),
		Latest:   stats.Latest.Unix(),
		NQueries: stats.NQueries,
		NErrors:  stats.NErrors,
	}
}