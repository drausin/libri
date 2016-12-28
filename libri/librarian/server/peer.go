package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"
)

var (
	IDUpperBound = big.NewInt(0)
	IDLowerBound = big.NewInt(0)
)

func init() {
	IDLowerBound.SetBytes(make([]byte, NodeIDLength))
	IDUpperBound.SetBytes(bytes.Repeat([]byte{255}, NodeIDLength))
}

// Peer represents a peer in the network.
type Peer struct {
	// 256-bit ID
	ID *big.Int

	// string encoding of ID
	IDStr string

	// self-reported name
	Name string

	// RPC TCP address
	PublicAddress *net.TCPAddr

	// time of latest response from the peer
	LatestResponse time.Time
}

func NewPeer(id *big.Int, name string, publicAddress *net.TCPAddr,
	latestResponse time.Time) (Peer, error) {
	if latestResponse.Location() != time.UTC {
		return Peer{}, errors.New(fmt.Sprintf("latestResponse should have a UTC location, "+
			"instead found %v", latestResponse.Location()))
	}

	return Peer{
		ID:             id,
		IDStr:          base64.URLEncoding.EncodeToString(id.Bytes()),
		Name:           name,
		PublicAddress:  publicAddress,
		LatestResponse: latestResponse,
	}, nil
}

func NewPeerFromStorage(stored StoredPeer) (Peer, error) {
	if len(stored.Id) != NodeIDLength {
		return Peer{}, errors.New(fmt.Sprintf("stored.Id (length %d) should have byte "+
			"length %d", NodeIDLength, len(stored.Id)))
	}
	var id *big.Int
	id.SetBytes(stored.Id)

	return NewPeer(
		id,
		stored.Name,
		&net.TCPAddr{
			IP:   net.ParseIP(stored.AddressIp),
			Port: int(stored.AddressPort),
		},
		time.Unix(stored.LatestResponse, int64(0)).UTC(),
	)
}
