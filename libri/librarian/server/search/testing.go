package search

import (
	"fmt"
	"math/rand"
	"net"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/gogo/protobuf/proto"
	"github.com/drausin/libri/libri/librarian/server/ecid"
)

type NoOpSigner struct {}

func (s *NoOpSigner) Sign(m proto.Message) (string, error) {
	return "noop.token.sig", nil
}

// TestConnector mocks the peer.Connector interface. The Connect() method returns a fixed client
// instead of creating one from the peer's address.
type TestConnector struct {
	addresses []*api.PeerAddress
}

// Connect is a no-op stub to satisfy the interface's signature.
func (c *TestConnector) Connect() (api.LibrarianClient, error) {
	return nil, nil
}

// Disconnect is a no-op stub to satisfy the interface's signature.
func (c *TestConnector) Disconnect() error {
	return nil
}

// TestFindQuerier mocks the FindQuerier interface. The Query() method returns a fixed
// api.FindPeersResponse, derived from a list of addresses in the client.
type TestFindQuerier struct{}

// Query mocks a real query to a peer, returning a fixed list of addresses stored in the
// TestConnector mock of the pConn peer.Connector.
func (c *TestFindQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	return &api.FindResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: rq.Metadata.RequestId,
		},
		Addresses: pConn.(*TestConnector).addresses,
	}, nil
}

// TestFromer mocks the Fromer interface. The FromAPI() method returns a pre-stored peer for that
// ID, allowing us to circumvent the creation of new peer.Peer and peer.Connector objects and use
// existing test peers with their testConnector (mocking peer.Connector) values instead.
type TestFromer struct {
	peers map[string]peer.Peer
}

// FromAPI mocks creating a new peer.Peer instance, instead looking up an existing peer stored
// in the TestFromer instance.
func (f *TestFromer) FromAPI(apiAddress *api.PeerAddress) peer.Peer {
	return f.peers[cid.FromBytes(apiAddress.PeerId).String()]
}

// NewTestSearcher creates a new Searcher instance with a FindQuerier and FindResponseProcessor that
// each just return fixed addresses and peers, respectively.
func NewTestSearcher(peersMap map[string]peer.Peer) Searcher {
	return NewSearcher(
		&NoOpSigner{},
		&TestFindQuerier{},
		&findResponseProcessor{
			peerFromer: &TestFromer{peers: peersMap},
		},
	)
}

// NewTestPeers creates a collection of test peers with fixed addresses in each's routing table
// (such that all find queries return the same addresses). It also returns the indices of the peers
// that peer 0 has in its routing table.
func NewTestPeers(rng *rand.Rand, n int) ([]peer.Peer, map[string]peer.Peer, []int, ecid.ID) {
	addresses := make([]*net.TCPAddr, n)
	ids := make([]cid.ID, n)
	names := make([]string, n)

	// create the addresses and IDs
	var selfID ecid.ID
	for i := 0; i < n; i++ {
		if i == 0 {
			selfID = ecid.NewPseudoRandom(rng)
			ids[0] = selfID.ID()
		} else {
			ids[i] = cid.NewPseudoRandom(rng)
		}
		names[i] = fmt.Sprintf("peer-%03d", i)
		address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", 11000+i))
		if err != nil {
			panic(err)
		}
		addresses[i] = address
	}

	// create our connections for each peer
	peers := make([]peer.Peer, n)
	peersMap := make(map[string]peer.Peer)
	selfPeerIdxs := make([]int, 0) // peer indices that peer 0 has in its routing table
	for i := 0; i < n; i++ {
		nConnectedPeers := rng.Int31n(8) + 3 // sample number of connected peers in [3, 8]
		connectedAddresses := make([]*api.PeerAddress, nConnectedPeers)
		for j := int32(0); j < nConnectedPeers; j++ {
			k := rng.Int31n(int32(n)) // sample a random peer to connect to
			connectedAddresses[j] = api.FromAddress(ids[k], names[k], addresses[k])
			if i == 0 {
				selfPeerIdxs = append(selfPeerIdxs, int(k))
			}
		}

		// create test connector with a test client that returns pre-determined set of
		// addresses
		conn := TestConnector{
			addresses: connectedAddresses,
		}
		peers[i] = peer.New(ids[i], names[i], &conn)
		peersMap[ids[i].String()] = peers[i]
	}

	return peers, peersMap, selfPeerIdxs, selfID
}