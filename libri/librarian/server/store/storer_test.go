package store

import (
	"math/rand"
	"testing"

	"errors"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"fmt"
)

func TestNewDefaultStorer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s := NewDefaultStorer(ecid.NewPseudoRandom(rng))
	assert.NotNil(t, s.(*storer).signer)
	assert.NotNil(t, s.(*storer).searcher)
	assert.NotNil(t, s.(*storer).storerCreator)
}

func TestStorer_Store_ok(t *testing.T) {
	n, nReplicas := 32, uint(3)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)

	// create our searcher
	value, key := api.NewTestDocument(rng)
	storer := &storer{
		searcher:      ssearch.NewTestSearcher(peersMap),
		storerCreator: &fixedStorerCreator{},
		signer:        &client.TestNoOpSigner{},
	}

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		searchParams := &ssearch.Parameters{
			NMaxErrors:  DefaultNMaxErrors,
			Concurrency: concurrency,
			Timeout:     DefaultQueryTimeout,
		}
		storeParams := &Parameters{
			NReplicas:   nReplicas,
			NMaxErrors:  DefaultNMaxErrors,
			Concurrency: concurrency,
		}
		store := NewStore(selfID, key, value, searchParams, storeParams)

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

		assert.True(t, uint(len(store.Result.Responded)) >= nReplicas)
		assert.True(t, uint(len(store.Result.Unqueried)) <= storeParams.NMaxErrors)
		assert.Equal(t, 0, len(store.Result.Errors))
		assert.Nil(t, store.Result.FatalErr)
	}
}

func TestStorer_Store_queryErr(t *testing.T) {
	storerImpl, store, selfPeerIdxs, peers, _ := newTestStore()
	seeds := ssearch.NewTestSeeds(peers, selfPeerIdxs)

	// mock storerCreator to always error
	storerImpl.(*storer).storerCreator = &fixedStorerCreator{
		err: errors.New("some Create error"),
	}

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
	assert.Equal(t, int(store.Params.NMaxErrors), len(store.Result.Errors))
	assert.Nil(t, store.Result.FatalErr)
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

func TestStorer_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	clientConn := api.NewConnector(nil) // won't actually be used since we're mocking the storer
	value, key := api.NewTestDocument(rng)
	selfID := ecid.NewPseudoRandom(rng)
	searchParams := &ssearch.Parameters{Timeout: DefaultQueryTimeout}
	store := NewStore(selfID, key, value, searchParams, &Parameters{})

	cases := []*storer{
		// case 0
		{
			signer:        &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{err: errors.New("some Create error")},
		},

		// case 1
		{
			signer:        &client.TestErrSigner{},
			storerCreator: &fixedStorerCreator{},
		},

		// case 2
		{
			signer: &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{
				storer: &fixedStorer{err: errors.New("some Store error")},
			},
		},

		// case 3
		{
			signer: &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{
				storer: &fixedStorer{requestID: []byte{1, 2, 3, 4}},
			},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp1, err := c.query(clientConn, store)
		assert.Nil(t, rp1, info)
		assert.NotNil(t, err, info)
	}
}

func newTestStore() (Storer, *Store, []int, []peer.Peer, cid.ID) {
	n := 32
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)

	// create our searcher
	value, key := api.NewTestDocument(rng)
	storerImpl := &storer{
		searcher:      ssearch.NewTestSearcher(peersMap),
		storerCreator: &fixedStorerCreator{},
		signer:        &client.TestNoOpSigner{},
	}

	concurrency := uint(1)
	searchParams := &ssearch.Parameters{
		NMaxErrors:  ssearch.DefaultNMaxErrors,
		Concurrency: concurrency,
		Timeout:     DefaultQueryTimeout,
	}
	storeParams := &Parameters{
		NReplicas:   DefaultNReplicas,
		NMaxErrors:  DefaultNMaxErrors,
		Concurrency: concurrency,
	}
	store := NewStore(selfID, key, value, searchParams, storeParams)

	return storerImpl, store, selfPeerIdxs, peers, key
}

type errSearcher struct{}

func (es *errSearcher) Search(search *ssearch.Search, seeds []peer.Peer) error {
	return errors.New("some search error")
}

type fixedStorerCreator struct {
	storer api.Storer
	err    error
}

func (c *fixedStorerCreator) Create(pConn api.Connector) (api.Storer, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.storer != nil {
		return c.storer, nil
	}
	return &fixedStorer{}, nil
}

type fixedStorer struct {
	requestID []byte
	err       error
}

func (f *fixedStorer) Store(ctx context.Context, rq *api.StoreRequest, opts ...grpc.CallOption) (
	*api.StoreResponse, error) {

	if f.err != nil {
		return nil, f.err
	}
	requestID := f.requestID
	if requestID == nil {
		requestID = rq.Metadata.RequestId
	}
	return &api.StoreResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: requestID,
		},
	}, nil
}
