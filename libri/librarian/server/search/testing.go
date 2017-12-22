package search

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// TestFinderCreator mocks the FindQuerier interface. The Create() method returns a fixed
// api.FindPeersResponse, derived from a list of addresses in the client.
type TestFinderCreator struct {
	finders map[string]api.Finder
	err    error
}

// Create creates an api.Finder that mocks a real query to a peer and returns a fixed list of
// addresses stored in the TestConnector mock of the pConn api.Connector.
func (c *TestFinderCreator) Create(address string) (api.Finder, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.finders[address], nil
}

type fixedFinder struct {
	addresses []*api.PeerAddress
	requestID []byte
	err       error
}

func (f *fixedFinder) Find(ctx context.Context, rq *api.FindRequest, opts ...grpc.CallOption) (
	*api.FindResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	requestID := f.requestID
	if requestID == nil {
		requestID = rq.Metadata.RequestId
	}
	return &api.FindResponse{
		Metadata: &api.ResponseMetadata{RequestId: requestID},
		Peers:    f.addresses,
	}, nil
}

// TestFromer mocks the Fromer interface. The FromAPI() method returns a pre-stored peer for that
// ID, allowing us to circumvent the creation of new peer.Peer and api.Connector objects and use
// existing test peers with their testConnector (mocking api.Connector) values instead.
type TestFromer struct {
	Peers map[string]peer.Peer
}

// FromAPI mocks creating a new peer.Peer instance, instead looking up an existing peer stored
// in the TestFromer instance.
func (f *TestFromer) FromAPI(apiAddress *api.PeerAddress) peer.Peer {
	return f.Peers[id.FromBytes(apiAddress.PeerId).String()]
}

// NewTestSearcher creates a new Searcher instance with a FindCreator and FindResponseProcessor that
// each just return fixed addresses and peers, respectively.
func NewTestSearcher(peersMap map[string]peer.Peer, addressFinders map[string]api.Finder) Searcher {
	return NewSearcher(
		&client.TestNoOpSigner{},
		&TestFinderCreator{finders: addressFinders},
		&responseProcessor{
			fromer: &TestFromer{Peers: peersMap},
		},
	)
}

// NewTestPeers creates a collection of test peers with fixed addresses in each's routing table
// (such that all find queries return the same addresses). It also returns the indices of the peers
// that peer 0 has in its routing table.
func NewTestPeers(rng *rand.Rand, n int) ([]peer.Peer, map[string]peer.Peer, []int, ecid.ID) {
	addresses := make([]*net.TCPAddr, n)
	ids := make([]id.ID, n)
	names := make([]string, n)
	addressFinders := make(map[string]api.Finder)

	// create the addresses and IDs
	var selfID ecid.ID
	for i := 0; i < n; i++ {
		if i == 0 {
			selfID = ecid.NewPseudoRandom(rng)
			ids[0] = selfID.ID()
		} else {
			ids[i] = id.NewPseudoRandom(rng)
		}
		names[i] = fmt.Sprintf("peer-%03d", i)
		address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", 20100+i))
		errors.MaybePanic(err)
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
			connectedAddresses[j] = peer.FromAddress(ids[k], names[k], addresses[k])
			if i == 0 {
				selfPeerIdxs = append(selfPeerIdxs, int(k))
			}
		}

		// create test connector with a test client that returns pre-determined set of
		// addresses
		peers[i] = peer.New(ids[i], names[i], addresses[i])
		peersMap[ids[i].String()] = peers[i]
		addressFinders[addresses[i].String()] = &fixedFinder{addresses: connectedAddresses}
	}

	return peers, peersMap, selfPeerIdxs, selfID
}

// NewTestSeeds creates a list of peers to use as seeds for a search.
func NewTestSeeds(peers []peer.Peer, selfPeerIdxs []int) []peer.Peer {
	// init the seeds of our search: usually this comes from the routing.Table.Peak()
	// method, but we'll just allocate directly
	seeds := make([]peer.Peer, len(selfPeerIdxs))
	for i := 0; i < len(selfPeerIdxs); i++ {
		seeds[i] = peers[selfPeerIdxs[i]]
	}
	return seeds
}
