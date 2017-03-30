package peer

import (
	"net"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
)

// FromStored creates a new peer.Peer instance from a storage.Peer instance.
func FromStored(stored *storage.Peer) Peer {
	conn := NewConnector(fromStoredAddress(stored.PublicAddress))
	return New(id.FromBytes(stored.Id), stored.Name, conn).(*peer).
		WithQueryRecorder(fromStoredQueryOutcomes(stored.QueryOutcomes))
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

// fromStoredQueryOutcomes creates a peer.Recorder from a storage.QueryOutcomes.
func fromStoredQueryOutcomes(stored *storage.QueryOutcomes) *queryRecorder {
	return &queryRecorder{
		responses: fromStoredQueryTypeOutcomes(stored.Responses),
		requests:  fromStoredQueryTypeOutcomes(stored.Requests),
	}
}

// fromStoredQueryTypeOutcomes creates a queryTypeOutcomes from a storage.QueryTypeOutcomes.
func fromStoredQueryTypeOutcomes(stored *storage.QueryTypeOutcomes) *queryTypeOutcomes {
	return &queryTypeOutcomes{
		earliest: time.Unix(stored.Earliest, int64(0)).UTC(),
		latest:   time.Unix(stored.Latest, int64(0)).UTC(),
		nQueries: stored.NQueries,
		nErrors:  stored.NErrors,
	}
}
