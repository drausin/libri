package peer

import (
	"net"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/storage"
)

// FromStored creates a new peer.Peer instance from a storage.Peer instance.
func FromStored(stored *storage.Peer) Peer {
	return New(
		id.FromBytes(stored.Id),
		stored.Name,
		fromStoredAddress(stored.PublicAddress),
	)
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
