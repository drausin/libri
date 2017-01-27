package api

import (
	"net"

	cid "github.com/drausin/libri/libri/common/id"
)

// ToAddress creates a net.TCPAddr from an api.PeerAddress.
func ToAddress(addr *PeerAddress) *net.TCPAddr {
	return &net.TCPAddr{
		IP:   net.ParseIP(addr.Ip),
		Port: int(addr.Port),
	}
}

// FromAddress creates an api.PeerAddress from a net.TCPAddr.
func FromAddress(id cid.ID, name string, addr *net.TCPAddr) *PeerAddress {
	return &PeerAddress{
		PeerId: id.Bytes(),
		PeerName: name,
		Ip:     addr.IP.String(),
		Port:   uint32(addr.Port),
	}
}
