package search

import (
	"net"
	"container/heap"
	"fmt"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
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

func (c *testClient) Find(ctx context.Context, in *api.FindRequest, opts ...grpc.CallOption) (
	*api.FindResponse, error) {
	return nil, nil
}

func (*testClient) Store(ctx context.Context, in *api.StoreRequest, opts ...grpc.CallOption) (
	*api.StoreResponse, error) {
	return nil, nil
}

// testQuerier mocks the Querier interface. The Query() method returns a fixed
// api.FindPeersResponse, derived from a list of addresses in the client.
type testQuerier struct{}

func (c *testQuerier) Query(client api.LibrarianClient, key cid.ID) (*api.FindResponse,
	error) {
	return &api.FindResponse{
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
	peers, peersMap, selfPeerIdxs := newTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	searcher := NewSearcher(
		&testQuerier{},
		&responseProcessor{
			peerFromer: &testFromer{peers: peersMap},
		},
	)

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		search := NewSearch(key, &Parameters{
			nClosestResponses: nClosestResponses,
			nMaxErrors:        DefaultNMaxErrors,
			concurrency:       concurrency,
			queryTimeout:      DefaultQueryTimeout,
		})

		// init the seeds of our search: usually this comes from the routing.Table.Peak()
		// method, but we'll just allocate directly
		seeds := make([]peer.Peer, len(selfPeerIdxs))
		for i := 0; i < len(selfPeerIdxs); i++ {
			seeds[i] = peers[selfPeerIdxs[i]]
		}

		// do the search!
		err := searcher.Search(search, seeds)

		// checks
		assert.Nil(t, err)
		assert.True(t, search.Finished())
		assert.True(t, search.FoundClosestPeers())
		assert.False(t, search.Errored())
		assert.False(t, search.Exhausted())
		assert.Equal(t, uint(0), search.nErrors)
		assert.Equal(t, int(nClosestResponses), search.result.closest.Len())
		assert.True(t, search.result.closest.Len() <= len(search.result.responded))

		// build set of closest peers by iteratively looking at all of them
		expectedClosestsPeers := make(map[string]struct{})
		farthestCloseDist := search.result.closest.PeakDistance()
		for _, p := range peers {
			pDist := key.Distance(p.ID())
			if pDist.Cmp(farthestCloseDist) <= 0 {
				expectedClosestsPeers[p.ID().String()] = struct{}{}
			}
		}

		// check all closest peers are in set of peers within farther close distance to
		// the key
		for search.result.closest.Len() > 0 {
			p := heap.Pop(search.result.closest).(peer.Peer)
			_, in := expectedClosestsPeers[p.ID().String()]
			assert.True(t, in)
		}
	}
}

func newTestPeers(rng *rand.Rand, n int) ([]peer.Peer, map[string]peer.Peer, []int) {
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
		conn := testConnector{
			client: &testClient{
				addresses: connectedAddresses,
			},
		}
		peers[i] = peer.New(ids[i], names[i], &conn)
		peersMap[ids[i].String()] = peers[i]
	}

	return peers, peersMap, selfPeerIdxs
}

// fixedFinder returns a fixed set of peer addresses for all find requests
type fixedFinder struct {
	addresses []*api.PeerAddress
}

func (f *fixedFinder) Find(ctx context.Context, client api.LibrarianClient, fr *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	return &api.FindResponse{
		RequestId: fr.RequestId,
		Addresses: f.addresses,
	}, nil
}

func TestFindQuerier_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))

	// create placeholder api.PeerAddresses for our mocked api.Find response
	nAddresses := 8

	// mock actual findPeers function to just return the peers we created above
	finder := &fixedFinder{addresses: newPeerAddresses(rng, nAddresses)}

	// querier that doesn't actually issue an api.FindPeers request, instead
	q := NewQuerier(NewParameters(), finder)
	c := &testClient{}

	rp, err := q.Query(c, cid.NewPseudoRandom(rng))
	assert.Nil(t, err)
	assert.NotNil(t, rp.RequestId)
	assert.Equal(t, nAddresses, len(rp.Addresses))
	assert.Nil(t, rp.Value)
}

// timeoutFinder returns an error simulating a request timeout
type timeoutFinder struct{}

func (f *timeoutFinder) Find(ctx context.Context, client api.LibrarianClient, fr *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	return nil, errors.New("simulated timeout error")
}

// diffRequestIDFinder returns a response with a different request ID
type diffRequestIDFinder struct {
	rng *rand.Rand
}

func (f *diffRequestIDFinder) Find(ctx context.Context, client api.LibrarianClient,
	fr *api.FindRequest, opts ...grpc.CallOption) (*api.FindResponse, error) {
	return &api.FindResponse{
		RequestId: cid.NewPseudoRandom(f.rng).Bytes(),
	}, nil
}

func TestFindQuerier_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	c := &testClient{}

	// check that the Find error propagates up to query
	q1 := NewQuerier(NewParameters(), &timeoutFinder{})
	rp1, err := q1.Query(c, cid.NewPseudoRandom(rng))
	assert.Nil(t, rp1)
	assert.NotNil(t, err)

	q2 := NewQuerier(NewParameters(), &diffRequestIDFinder{rng})
	rp2, err := q2.Query(c, cid.NewPseudoRandom(rng))
	assert.Nil(t, rp2)
	assert.NotNil(t, err)
}

func TestResponseProcessor_Process_Value(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := cid.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	result := NewInitialResult(key, NewParameters())

	// create response with the value
	value := cid.NewPseudoRandom(rng).Bytes() // random value
	response2 := &api.FindResponse{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Addresses: nil,
		Value:     value,
	}

	// check that the result value is set
	prevUnqueriedLength := result.unqueried.Len()
	err := rp.Process(response2, result)
	assert.Nil(t, err)
	assert.Equal(t, prevUnqueriedLength, result.unqueried.Len())
	assert.Equal(t, value, result.value)
}

func TestResponseProcessor_Process_Addresses(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))

	// create placeholder api.PeerAddresses for our mocked api.FindPeers response
	nAddresses1 := 8
	peerAddresses1 := newPeerAddresses(rng, nAddresses1)

	key := cid.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	result := NewInitialResult(key, NewParameters())

	// create response or nAddresses and process it
	response1 := &api.FindResponse{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Addresses: peerAddresses1,
		Value:     nil,
	}
	err := rp.Process(response1, result)
	assert.Nil(t, err)

	// check that all responses have gone into the unqueried heap
	assert.Equal(t, nAddresses1, result.unqueried.Len())

	// process same response as before and check that the length of unqueried hasn't changed
	err = rp.Process(response1, result)
	assert.Nil(t, err)
	assert.Equal(t, nAddresses1, result.unqueried.Len())

	// create new peers and add them to the closest heap (as if we'd already heard from them)
	nAddresses2 := 3
	peerAddresses2 := newPeerAddresses(rng, nAddresses2)
	peerFromer := peer.NewFromer()
	for _, pa := range peerAddresses2 {
		err = result.closest.SafePush(peerFromer.FromAPI(pa))
		assert.Nil(t, err)
	}

	// check that a response with these peers has no effect
	response2 := &api.FindResponse{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Addresses: peerAddresses2,
		Value:     nil,
	}
	err = rp.Process(response2, result)
	assert.Nil(t, err)
	assert.Equal(t, nAddresses1, result.unqueried.Len())
	assert.Equal(t, nAddresses2, result.closest.Len())
}

func TestResponseProcessor_Process_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := cid.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	result := NewInitialResult(key, NewParameters())

	// create a bad response with neither a value nor peer addresses
	response2 := &api.FindResponse{
		RequestId: cid.NewPseudoRandom(rng).Bytes(),
		Addresses: nil,
		Value:     nil,
	}
	err := rp.Process(response2, result)
	assert.NotNil(t, err)
}

func newPeerAddresses(rng *rand.Rand, n int) []*api.PeerAddress {
	peerAddresses := make([]*api.PeerAddress, n)
	for i := 0; i < n; i++ {
		peerAddresses[i] = &api.PeerAddress{
			PeerId:   cid.NewPseudoRandom(rng).Bytes(),
			PeerName: fmt.Sprintf("peer-%03d", i),
			Ip:       "localhost",
			Port:     uint32(11000 + i),
		}
	}
	return peerAddresses
}
