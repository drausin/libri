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
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestNewDefaultStorer(t *testing.T) {
	s := NewDefaultStorer(
		&client.TestNoOpSigner{},
		&client.TestNoOpSigner{},
		&fixedRecorder{},
		comm.NewNaiveDoctor(),
		nil,
	)
	assert.NotNil(t, s.(*storer).peerSigner)
	assert.NotNil(t, s.(*storer).orgSigner)
	assert.NotNil(t, s.(*storer).searcher)
	assert.NotNil(t, s.(*storer).storerCreator)
}

func TestStorer_Store_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nReplicas := uint(3)
	nPeers := []int{3, 8, 16, 32}
	concurrencies := []uint{1, 2, 3}

	for _, n := range nPeers {
		peers, peersMap, addressFinders, selfPeerIdxs, peerID := ssearch.NewTestPeers(rng, n)
		orgID := ecid.NewPseudoRandom(rng)

		// create our storer
		value, key := api.NewTestDocument(rng)
		rec := &fixedRecorder{}
		storer := &storer{
			searcher:      ssearch.NewTestSearcher(peersMap, addressFinders, rec),
			storerCreator: &fixedStorerCreator{},
			peerSigner:    &client.TestNoOpSigner{},
			orgSigner:     &client.TestNoOpSigner{},
			rec:           rec,
			doc:           comm.NewNaiveDoctor(),
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
			store := NewStore(peerID, orgID, key, value, searchParams, storeParams)

			// init the seeds of our search: usually this comes from the routing.Table.Peak()
			// method, but we'll just allocate directly
			seeds := make([]peer.Peer, len(selfPeerIdxs))
			for i := 0; i < len(selfPeerIdxs); i++ {
				seeds[i] = peers[selfPeerIdxs[i]]
			}

			// do the store!
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
			assert.True(t, len(store.Result.Responded) <= rec.nSuccesses)
			assert.Equal(t, 0, rec.nErrors)
		}
	}
}

func TestStorer_Store_queryErr(t *testing.T) {
	rec := &fixedRecorder{}
	storerImpl, store, selfPeerIdxs, peers, _ := newTestStore(rec)
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
	assert.True(t, rec.nErrors > 0)
}

func TestStorer_Store_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	s := &storer{
		searcher: &errSearcher{},
	}

	// check that Store() surfaces searcher error
	peerID, key := ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	store := &Store{
		Result: &Result{},
		Params: NewDefaultParameters(),
		Search: ssearch.NewSearch(peerID, orgID, key, ssearch.NewDefaultParameters()),
	}
	assert.NotNil(t, s.Store(store, []peer.Peer{}))
}

func TestStorer_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	value, key := api.NewTestDocument(rng)
	peerID := ecid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	searchParams := &ssearch.Parameters{Timeout: DefaultQueryTimeout}
	store := NewStore(peerID, orgID, key, value, searchParams, &Parameters{})

	cases := []*storer{
		// case 0
		{
			peerSigner:    &client.TestNoOpSigner{},
			orgSigner:     &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{err: errors.New("some Create error")},
		},

		// case 1
		{
			peerSigner:    &client.TestErrSigner{},
			orgSigner:     &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{},
		},

		// case 2
		{
			peerSigner: &client.TestNoOpSigner{},
			orgSigner:  &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{
				storer: &fixedStorer{err: errors.New("some Store error")},
			},
		},

		// case 3
		{
			peerSigner: &client.TestNoOpSigner{},
			orgSigner:  &client.TestNoOpSigner{},
			storerCreator: &fixedStorerCreator{
				storer: &fixedStorer{requestID: []byte{1, 2, 3, 4}},
			},
		},
	}
	next := peer.NewTestPeer(rng, 0)
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp1, err := c.query(next, store)
		assert.Nil(t, rp1, info)
		assert.NotNil(t, err, info)
	}
}

func newTestStore(rec comm.QueryRecorder) (Storer, *Store, []int, []peer.Peer, cid.ID) {
	n := 32
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, addressFinders, selfPeerIdxs, peerID := ssearch.NewTestPeers(rng, n)
	orgID := ecid.NewPseudoRandom(rng)

	// create our searcher
	value, key := api.NewTestDocument(rng)
	storerImpl := &storer{
		searcher:      ssearch.NewTestSearcher(peersMap, addressFinders, rec),
		storerCreator: &fixedStorerCreator{},
		peerSigner:    &client.TestNoOpSigner{},
		rec:           rec,
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
	store := NewStore(peerID, orgID, key, value, searchParams, storeParams)

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

func (c *fixedStorerCreator) Create(address string) (api.Storer, error) {
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

type fixedRecorder struct {
	nSuccesses int
	nErrors    int
}

func (f *fixedRecorder) Record(
	peerID cid.ID, endpoint api.Endpoint, qt comm.QueryType, o comm.Outcome,
) {
	if o == comm.Success {
		f.nSuccesses++
	} else {
		f.nErrors++
	}
}

func (f *fixedRecorder) Get(peerID cid.ID, endpoint api.Endpoint) comm.QueryOutcomes {
	panic("implement me")
}

func (f *fixedRecorder) CountPeers(endpoint api.Endpoint, qt comm.QueryType, known bool) int {
	panic("implement me")
}
