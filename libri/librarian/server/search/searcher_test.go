package search

import (
	"container/heap"
	"fmt"
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"time"
)

func TestNewDefaultSearcher(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s := NewDefaultSearcher(ecid.NewPseudoRandom(rng))
	assert.NotNil(t, s.(*searcher).signer)
	assert.NotNil(t, s.(*searcher).querier)
	assert.NotNil(t, s.(*searcher).rp)
}

func TestSearcher_Search(t *testing.T) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := NewTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	searcher := NewTestSearcher(peersMap)

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		search := NewSearch(selfID, key, &Parameters{
			NClosestResponses: nClosestResponses,
			NMaxErrors:        DefaultNMaxErrors,
			Concurrency:       concurrency,
			Timeout:           DefaultQueryTimeout,
		})

		seeds := NewTestSeeds(peers, selfPeerIdxs)

		// do the search!
		err := searcher.Search(search, seeds)

		// checks
		assert.Nil(t, err)
		assert.True(t, search.Finished())
		assert.True(t, search.FoundClosestPeers())
		assert.False(t, search.Errored())
		assert.False(t, search.Exhausted())
		assert.Equal(t, uint(0), search.Result.NErrors)
		assert.Equal(t, int(nClosestResponses), search.Result.Closest.Len())
		assert.True(t, search.Result.Closest.Len() <= len(search.Result.Responded))

		// build set of closest peers by iteratively looking at all of them
		expectedClosestsPeers := make(map[string]struct{})
		farthestCloseDist := search.Result.Closest.PeakDistance()
		for _, p := range peers {
			pDist := key.Distance(p.ID())
			if pDist.Cmp(farthestCloseDist) <= 0 {
				expectedClosestsPeers[p.ID().String()] = struct{}{}
			}
		}

		// check all closest peers are in set of peers within farther close distance to
		// the key
		for search.Result.Closest.Len() > 0 {
			p := heap.Pop(search.Result.Closest).(peer.Peer)
			_, in := expectedClosestsPeers[p.ID().String()]
			assert.True(t, in)
		}
	}
}

func TestSearcher_Search_connectorErr(t *testing.T) {
	searcher, search, selfPeerIdxs, peers := newTestSearch()
	for i, p := range peers {
		// replace peer with connector that errors
		peers[i] = peer.New(p.ID(), "", &TestErrConnector{})
	}

	seeds := NewTestSeeds(peers, selfPeerIdxs)

	// do the search!
	err := searcher.Search(search, seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, search.Exhausted()) // since we can't connect to any of the peers
	assert.True(t, search.Finished())
	assert.False(t, search.FoundClosestPeers())
	assert.False(t, search.Errored())
	assert.Equal(t, uint(0), search.Result.NErrors)
	assert.Equal(t, 0, search.Result.Closest.Len())
	assert.Equal(t, 0, len(search.Result.Responded))
}

func TestSearcher_Search_queryErr(t *testing.T) {
	searcherImpl, search, selfPeerIdxs, peers := newTestSearch()
	seeds := NewTestSeeds(peers, selfPeerIdxs)

	// all queries return errors as if they'd timed out
	searcherImpl.(*searcher).querier = &timeoutQuerier{}

	// do the search!
	err := searcherImpl.Search(search, seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, search.Errored())    // since all of the queries return errors
	assert.False(t, search.Exhausted()) // since NMaxErrors < len(Upqueried)
	assert.True(t, search.Finished())
	assert.False(t, search.FoundClosestPeers())
	assert.Equal(t, search.Params.NMaxErrors, search.Result.NErrors)
	assert.Equal(t, 0, search.Result.Closest.Len())
	assert.True(t, 0 < search.Result.Unqueried.Len())
	assert.Equal(t, 0, len(search.Result.Responded))
}

type errResponseProcessor struct{}

func (erp *errResponseProcessor) Process(rp *api.FindResponse, result *Result) error {
	return errors.New("some fatal processing error")
}

func TestSearcher_Search_rpErr(t *testing.T) {
	searcherImpl, search, selfPeerIdxs, peers := newTestSearch()
	seeds := NewTestSeeds(peers, selfPeerIdxs)

	// mock some internal issue when processing responses
	searcherImpl.(*searcher).rp = &errResponseProcessor{}

	// do the search!
	err := searcherImpl.Search(search, seeds)

	// checks
	assert.NotNil(t, err)
	assert.NotNil(t, search.Result.FatalErr)
	assert.True(t, search.Errored()) // since we got a fatal error while processing responses
	assert.False(t, search.Exhausted())
	assert.True(t, search.Finished())
	assert.False(t, search.FoundClosestPeers())
	assert.Equal(t, uint(0), search.Result.NErrors)
	assert.Equal(t, 0, search.Result.Closest.Len())
	assert.True(t, 0 < search.Result.Unqueried.Len())
	assert.Equal(t, 0, len(search.Result.Responded))
}

func newTestSearch() (Searcher, *Search, []int, []peer.Peer) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := NewTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	searcher := NewTestSearcher(peersMap)

	search := NewSearch(selfID, key, &Parameters{
		NClosestResponses: nClosestResponses,
		NMaxErrors:        DefaultNMaxErrors,
		Concurrency:       uint(1),
		Timeout:           DefaultQueryTimeout,
	})
	return searcher, search, selfPeerIdxs, peers
}

// fixedFinder returns a fixed set of peer addresses for all find requests
type fixedQuerier struct {
	peerID    ecid.ID
	addresses []*api.PeerAddress
}

func (f *fixedQuerier) Query(ctx context.Context, pConn peer.Connector, fr *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	return &api.FindResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: fr.Metadata.RequestId,
			PubKey:    ecid.ToPublicKeyBytes(f.peerID),
		},
		Addresses: f.addresses,
	}, nil
}

func TestSearcher_query_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	nAddresses := 8
	peerID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	search := NewSearch(peerID, key, &Parameters{
		NClosestResponses: uint(nAddresses),
		Timeout:           DefaultQueryTimeout,
	})
	s := &searcher{
		signer: &TestNoOpSigner{},
		// use querier that returns fixed set of addresses
		querier: &fixedQuerier{
			peerID:    peerID,
			addresses: newPeerAddresses(rng, nAddresses),
		},
		rp: nil,
	}
	client := peer.NewConnector(nil) // won't actually be uses since we're mocking the finder

	rp, err := s.query(client, search)
	assert.Nil(t, err)
	assert.NotNil(t, rp.Metadata.RequestId)
	assert.NotNil(t, rp.Metadata.PubKey)
	assert.Equal(t, nAddresses, len(rp.Addresses))
	assert.Nil(t, rp.Value)
}

// timeoutQuerier returns an error simulating a request timeout
type timeoutQuerier struct{}

func (f *timeoutQuerier) Query(ctx context.Context, pConn peer.Connector, fr *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindResponse, error) {
	return nil, errors.New("simulated timeout error")
}

// diffRequestIDFinder returns a response with a different request ID
type diffRequestIDQuerier struct {
	rng    *rand.Rand
	peerID ecid.ID
}

func (f *diffRequestIDQuerier) Query(ctx context.Context, pConn peer.Connector,
	fr *api.FindRequest, opts ...grpc.CallOption) (*api.FindResponse, error) {
	return &api.FindResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: cid.NewPseudoRandom(f.rng).Bytes(),
			PubKey:    ecid.ToPublicKeyBytes(f.peerID),
		},
	}, nil
}

func TestSearcher_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	client := peer.NewConnector(nil) // won't actually be used since we're mocking the finder
	peerID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	search := NewSearch(peerID, key, &Parameters{
		Timeout: DefaultQueryTimeout,
	})

	s1 := &searcher{
		signer: &TestNoOpSigner{},
		// use querier that simulates a timeout
		querier: &timeoutQuerier{},
		rp:      nil,
	}
	rp1, err := s1.query(client, search)
	assert.Nil(t, rp1)
	assert.NotNil(t, err)

	s2 := &searcher{
		signer: &TestNoOpSigner{},
		// use querier that simulates a timeout
		querier: &diffRequestIDQuerier{
			rng:    rng,
			peerID: peerID,
		},
		rp: nil,
	}
	rp2, err := s2.query(client, search)
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
		Addresses: nil,
		Value:     value,
	}

	// check that the result value is set
	prevUnqueriedLength := result.Unqueried.Len()
	err := rp.Process(response2, result)
	assert.Nil(t, err)
	assert.Equal(t, prevUnqueriedLength, result.Unqueried.Len())
	assert.Equal(t, value, result.Value)
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
		Addresses: peerAddresses1,
		Value:     nil,
	}
	err := rp.Process(response1, result)
	assert.Nil(t, err)

	// check that all responses have gone into the unqueried heap
	assert.Equal(t, nAddresses1, result.Unqueried.Len())

	// process same response as before and check that the length of unqueried hasn't changed
	err = rp.Process(response1, result)
	assert.Nil(t, err)
	assert.Equal(t, nAddresses1, result.Unqueried.Len())

	// create new peers and add them to the closest heap (as if we'd already heard from them)
	nAddresses2 := 3
	peerAddresses2 := newPeerAddresses(rng, nAddresses2)
	peerFromer := peer.NewFromer()
	for _, pa := range peerAddresses2 {
		err = result.Closest.SafePush(peerFromer.FromAPI(pa))
		assert.Nil(t, err)
	}

	// check that a response with these peers has no effect
	response2 := &api.FindResponse{
		Addresses: peerAddresses2,
		Value:     nil,
	}
	err = rp.Process(response2, result)
	assert.Nil(t, err)
	assert.Equal(t, nAddresses1, result.Unqueried.Len())
	assert.Equal(t, nAddresses2, result.Closest.Len())
}

func TestResponseProcessor_Process_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := cid.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	result := NewInitialResult(key, NewParameters())

	// create a bad response with neither a value nor peer addresses
	response2 := &api.FindResponse{
		Addresses: nil,
		Value:     nil,
	}
	err := rp.Process(response2, result)
	assert.NotNil(t, err)
}

func TestNewSignedTimeoutContext_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	ctx, cancel, err := NewSignedTimeoutContext(
		&TestNoOpSigner{},
		api.NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), 20),
		5*time.Second,
	)
	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.Value(signature.ContextKey))
	assert.NotNil(t, cancel)
	assert.Nil(t, err)
}

func TestNewSignedTimeoutContext_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	ctx, cancel, err := NewSignedTimeoutContext(
		&TestErrSigner{},
		api.NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), 20),
		5*time.Second,
	)
	assert.Nil(t, ctx)
	assert.Nil(t, cancel)
	assert.NotNil(t, err)
}

func TestQuerier_Query_err(t *testing.T) {
	c := &TestErrConnector{}
	q := NewQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}

// Explanation: ideally would have unit test like this, but mocking an api.LibrarianClient is
// annoying b/c there are so many service methods. Will have to rely on integration tests to cover
// this branch.
//
// func TestQuerier_Query_ok(t *testing.T) {}

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
