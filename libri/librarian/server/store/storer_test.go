package store

import (
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/drausin/libri/libri/librarian/signature"
)

func TestNewDefaultStorer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s := NewDefaultStorer(ecid.NewPseudoRandom(rng))
	assert.NotNil(t, s.(*storer).signer)
	assert.NotNil(t, s.(*storer).searcher)
	assert.NotNil(t, s.(*storer).querier)
}

// TestStoreQuerier mocks the StoreQuerier interface. The Query() method returns an
// api.StoreResponse, as if the remote peer had successfully stored the value.
type TestStoreQuerier struct {
	peerID ecid.ID
}

func (c *TestStoreQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	return &api.StoreResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: rq.Metadata.RequestId,
			PubKey:    ecid.ToPublicKeyBytes(c.peerID),
		},
	}, nil
}

func NewTestStorer(peerID ecid.ID, peersMap map[string]peer.Peer) Storer {
	return &storer{
		searcher: ssearch.NewTestSearcher(peersMap),
		querier:  &TestStoreQuerier{peerID: peerID},
		signer:   &signature.TestNoOpSigner{},
	}
}

func TestStorer_Store_ok(t *testing.T) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything
	storer := NewTestStorer(selfID, peersMap)

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		search := ssearch.NewSearch(selfID, key, &ssearch.Parameters{
			NClosestResponses: nClosestResponses,
			NMaxErrors:        DefaultNMaxErrors,
			Concurrency:       concurrency,
			Timeout:           DefaultQueryTimeout,
		})
		store := NewStore(selfID, search, value, &Parameters{
			NMaxErrors:  DefaultNMaxErrors,
			Concurrency: concurrency,
		})

		// init the seeds of our search: usually this comes from the routing.Table.Peak()
		// method, but we'll just allocate directly
		seeds := make([]peer.Peer, len(selfPeerIdxs))
		for i := 0; i < len(selfPeerIdxs); i++ {
			seeds[i] = peers[selfPeerIdxs[i]]
		}

		// do the search!
		err := storer.Store(store, seeds)

		// checks
		assert.Nil(t, err)
		assert.True(t, store.Finished())
		assert.True(t, store.Stored())
		assert.False(t, store.Errored())

		assert.Equal(t, nClosestResponses, uint(len(store.Result.Responded)))
		assert.Equal(t, 0, len(store.Result.Unqueried))
		assert.Equal(t, uint(0), store.Result.NErrors)
		assert.Nil(t, store.Result.FatalErr)
	}
}

type fixedSearcher struct {
	fixed *ssearch.Result
}

func (s *fixedSearcher) Search(search *ssearch.Search, seeds []peer.Peer) error {
	search.Result = s.fixed
	return nil
}

func TestStorer_Store_connectorErr(t *testing.T) {
	storerImpl, store, selfPeerIdxs, peers, key := newTestStore()
	seeds := ssearch.NewTestSeeds(peers, selfPeerIdxs)

	// define fixed closest peers in search result, used by mocked searcher
	fixed := ssearch.NewInitialResult(key, store.Search.Params)
	for c := uint(0); c < store.Search.Params.NClosestResponses; c++ {
		badPeer := peer.New(peers[c].ID(), "", &peer.TestErrConnector{})
		err := fixed.Closest.SafePush(badPeer)
		assert.Nil(t, err)
	}
	storerImpl.(*storer).searcher = &fixedSearcher{
		fixed: fixed,
	}

	// do the search!
	err := storerImpl.Store(store, seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, store.Exhausted()) // since we can't connect to any of the peers
	assert.False(t, store.Stored())
	assert.False(t, store.Errored())

	assert.Equal(t, 0, len(store.Result.Responded))
	assert.Equal(t, 0, len(store.Result.Unqueried))
	assert.Equal(t, uint(0), store.Result.NErrors)
	assert.Nil(t, store.Result.FatalErr)
}

func TestStorer_Store_queryErr(t *testing.T) {
	storerImpl, store, selfPeerIdxs, peers, _ := newTestStore()
	seeds := ssearch.NewTestSeeds(peers, selfPeerIdxs)

	// mock querier to always timeout
	storerImpl.(*storer).querier = &timeoutQuerier{}

	// do the search!
	err := storerImpl.Store(store, seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, store.Errored())    // since all of the queries return errors
	assert.False(t, store.Exhausted()) // since NMaxErrors < len(Unqueried)
	assert.False(t, store.Stored())
	assert.True(t, store.Finished())

	assert.Equal(t, 0, len(store.Result.Responded))
	assert.True(t, 0 < len(store.Result.Unqueried))
	assert.Equal(t, store.Params.NMaxErrors, store.Result.NErrors)
	assert.Nil(t, store.Result.FatalErr)
}

func newTestStore() (Storer, *Store, []int, []peer.Peer, cid.ID) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything
	storerImpl := NewTestStorer(selfID, peersMap)

	concurrency := uint(1)
	search := ssearch.NewSearch(selfID, key, &ssearch.Parameters{
		NClosestResponses: nClosestResponses,
		NMaxErrors:        DefaultNMaxErrors,
		Concurrency:       concurrency,
		Timeout:           DefaultQueryTimeout,
	})
	store := NewStore(selfID, search, value, &Parameters{
		NMaxErrors:  DefaultNMaxErrors,
		Concurrency: concurrency,
	})

	return storerImpl, store, selfPeerIdxs, peers, key
}

type errSearcher struct{}

func (es *errSearcher) Search(search *ssearch.Search, seeds []peer.Peer) error {
	return errors.New("some search error")
}

func TestStorer_Store_err(t *testing.T) {
	s := &storer{
		searcher: &errSearcher{},
	}

	// check that Store() surfaces searcher error
	store := &Store{
		Result: &Result{},
	}
	assert.NotNil(t, s.Store(store, nil))
}

// timeoutQuerier returns an error simulating a request timeout
type timeoutQuerier struct{}

func (f *timeoutQuerier) Query(ctx context.Context, pConn peer.Connector, fr *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	return nil, errors.New("simulated timeout error")
}

// diffRequestIDFinder returns a response with a different request ID
type diffRequestIDQuerier struct {
	rng    *rand.Rand
	peerID ecid.ID
}

func (f *diffRequestIDQuerier) Query(ctx context.Context, pConn peer.Connector,
	fr *api.StoreRequest, opts ...grpc.CallOption) (*api.StoreResponse, error) {
	return &api.StoreResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: cid.NewPseudoRandom(f.rng).Bytes(),
			PubKey:    ecid.ToPublicKeyBytes(f.peerID),
		},
	}, nil
}

func TestStorer_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	client := peer.NewConnector(nil) // won't actually be used since we're mocking the finder
	selfID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything
	search := ssearch.NewSearch(selfID, key, &ssearch.Parameters{
		Timeout: DefaultQueryTimeout,
	})
	store := NewStore(selfID, search, value, &Parameters{})

	s1 := &storer{
		signer: &signature.TestNoOpSigner{},
		// use querier that simulates a timeout
		querier: &timeoutQuerier{},
	}
	rp1, err := s1.query(client, store)
	assert.Nil(t, rp1)
	assert.NotNil(t, err)

	s2 := &storer{
		signer: &signature.TestNoOpSigner{},
		// use querier that simulates a different request ID
		querier: &diffRequestIDQuerier{
			rng:    rng,
			peerID: selfID,
		},
	}
	rp2, err := s2.query(client, store)
	assert.Nil(t, rp2)
	assert.NotNil(t, err)

	s3 := &storer{
		// use signer that returns an error
		signer: &signature.TestErrSigner{},
	}
	rp3, err := s3.query(client, store)
	assert.Nil(t, rp3)
	assert.NotNil(t, err)
}
