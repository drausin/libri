package search

import (
	"net"

	"github.com/drausin/libri/libri/librarian/api"
	cid "github.com/drausin/libri/libri/common/id"
	"fmt"
	"testing"
	"math/rand"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"google.golang.org/grpc"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"container/heap"
)

// testConnector mocks the peer.Connector interface. The Connect() method returns a fixed client
// instead of creating one from the peer's address.
type testConnector struct {
	client api.LibrarianClient
}

func (c *testConnector) Connect() (api.LibrarianClient, error) {
	return c.client, nil
}

func (c *testConnector) Disconnect() error {
	return nil
}

// testClient mocks the api.LibrarianClient interface and is used by the testQuerier below to
// return a fixed list of api.PeerAddresses. All methods are just stubs.
type testClient struct {
	addresses []*api.PeerAddress
}

func (c *testClient) Ping(ctx context.Context, in *api.PingRequest, opts ...grpc.CallOption) (
	*api.PingResponse, error) {
	return nil, nil
}

func (c *testClient) Identify(ctx context.Context, in *api.IdentityRequest,
	opts ...grpc.CallOption) (*api.IdentityResponse, error) {
	return nil, nil
}

func (c *testClient) FindPeers(ctx context.Context, in *api.FindRequest, opts ...grpc.CallOption) (
	*api.FindPeersResponse, error) {
	return nil, nil
}

// testQuerier mocks the Querier interface. The Query() method returns a fixed
// api.FindPeersResponse, derived from a list of addresses in the client.
type testQuerier struct {}

func (c *testQuerier) Query(client api.LibrarianClient) (*api.FindPeersResponse, error) {
	return &api.FindPeersResponse{
		RequestId: nil,
		Addresses: client.(*testClient).addresses,
	}, nil
}

// testFromer mocks the Fromer interface. The FromAPI() method returns a pre-stored peer for that
// ID, allowing us to circumvent the creation of new peer.Peer and peer.Connector objects and use
// existing test peers with their testConnector (mocking peer.Connector) values instead.
type testFromer struct {
	peers map[string]peer.Peer
}

func (f *testFromer) FromAPI(apiAddress *api.PeerAddress) peer.Peer {
	return f.peers[cid.FromBytes(apiAddress.PeerId).String()]
}

func TestSearch(t *testing.T) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	addresses := make([]*net.TCPAddr, n)
	ids := make([]cid.ID, n)
	names := make([]string, n)

	// create the addresses and IDs
	for i := 0; i < n; i++ {
		ids[i] = cid.NewPseudoRandom(rng)
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
	selfPeerIdxs := make([]int, 0)  // peer indices that peer 0 has in its routing table
	for i := 0; i < n; i++ {
		nConnectedPeers := rng.Int31n(8) + 3  // sample number of connected peers in [3, 8]
		connectedAddresses := make([]*api.PeerAddress, nConnectedPeers)
		for j := int32(0); j < nConnectedPeers; j++ {
			k := rng.Int31n(int32(n))  // sample a random peer to connect to
			connectedAddresses[j] = api.FromAddress(ids[k], names[k], addresses[k])
			if i == 0 {
				selfPeerIdxs = append(selfPeerIdxs, int(k))
			}
		}

		// create test connector with a test client that returns pre-determined set of
		// addresses
		conn := testConnector{
			client: &testClient{
				addresses: connectedAddresses,
			},
		}
		peers[i] = peer.New(ids[i], names[i], &conn)
		peersMap[ids[i].String()] = peers[i]
	}

	// create our searcher
	target := cid.NewPseudoRandom(rng)
	s := NewSearcher(target).
		WithNClosestResponses(nClosestResponses).
		WithQuerier(&testQuerier{}).
		WithResponseProcessor(
			&responseProcessor{
				peerFromer: &testFromer{peers: peersMap},
			},
		)

	// init the seeds of our search: usually this comes from the routing.Table.Peak()
	// method, but we'll just allocate directly
	seeds := make([]peer.Peer, len(selfPeerIdxs))
	for i := 0; i < len(selfPeerIdxs); i++ {
		seeds[i] = peers[selfPeerIdxs[i]]
	}

	// do the search!
	result, err := s.Search(seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, result.found)
	assert.Equal(t, int(nClosestResponses), result.closest.Len())
	assert.True(t, result.closest.Len() >= len(result.responded))

	// build set of closest peers by iteratively looking at all of them
	expectedClosestsPeers := make(map[string]struct{})
	farthestCloseDist := result.closest.PeakDistance()
	for _, p := range peers {
		pDist := target.Distance(p.ID())
		if pDist.Cmp(farthestCloseDist) <= 0 {
			expectedClosestsPeers[p.ID().String()] = struct{}{}
		}
	}

	// check all closest peers are in set of peers within farther close distance to the target
	for result.closest.Len() > 0 {
		p := heap.Pop(result.closest).(peer.Peer)
		_, in := expectedClosestsPeers[p.ID().String()]
		assert.True(t, in)
	}
}
