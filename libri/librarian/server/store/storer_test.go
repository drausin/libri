package store

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestNewDefaultStorer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s := NewDefaultStorer(ecid.NewPseudoRandom(rng))
	assert.NotNil(t, s.(*storer).signer)
	assert.NotNil(t, s.(*storer).searcher)
	assert.NotNil(t, s.(*storer).storerCreator)
}

func TestStorer_Store_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nReplicas := uint(3)
	nPeers := []int{3, 8, 16, 32}
	concurrencies := []uint{1, 2, 3}

	for _, n := range nPeers {
		peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)

		// create our searcher
		value, key := api.NewTestDocument(rng)
		storer := &storer{
			searcher:      ssearch.NewTestSearcher(peersMap),
			storerCreator: &fixedStorerCreator{},
			signer:        &client.TestNoOpSigner{},
		}

		for _, concurrency := range concurrencies {
			info := fmt.Sprintf("nPeers: %d, concurrency: %d", n, concurrency)
			//log.Printf("current case: %s", info)  // sometimes helpful for debugging

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
			assert.Nil(t, err, info)
			assert.True(t, store.Finished(), info)
			assert.True(t, store.Stored(), info)
			assert.False(t, store.Errored(), info)

			assert.Equal(t, nReplicas, uint(len(store.Result.Responded)), info)
			assert.True(t, storeParams.NMaxErrors >= uint(len(store.Result.Unqueried)), info)
			assert.Equal(t, 0, len(store.Result.Errors), info)
			assert.Nil(t, store.Result.FatalErr, info)
		}
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
	assert.NotNil(t, err)
	assert.True(t, store.Finished())
	assert.True(t, store.Errored())    // since all of the queries return errors
	assert.False(t, store.Exhausted()) // since NMaxErrors < len(Unqueried)
	assert.False(t, store.Stored())

	assert.Equal(t, 0, len(store.Result.Responded))
	assert.True(t, 3 >= len(store.Result.Unqueried))
	assert.True(t, int(store.Params.NMaxErrors) <= len(store.Result.Errors))
	assert.Equal(t, ErrTooManyStoreErrors, store.Result.FatalErr)
}

func TestStorer_Store_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	s := &storer{
		searcher: &errSearcher{},
	}

	// check that Store() surfaces searcher error
	selfID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	store := &Store{
		Result: &Result{},
		Params: NewDefaultParameters(),
		Search: ssearch.NewSearch(selfID, key, ssearch.NewDefaultParameters()),
	}
	assert.NotNil(t, s.Store(store, []peer.Peer{}))
}

func TestStorer_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	clientConn := peer.NewConnector(nil) // won't actually be used since we're mocking the storer
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

func (c *fixedStorerCreator) Create(pConn peer.Connector) (api.Storer, error) {
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
