package search

import (
	"container/heap"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultSearcher(t *testing.T) {
	s := NewDefaultSearcher(
		&client.TestNoOpSigner{},
		&client.TestNoOpSigner{},
		&fixedRecorder{},
		comm.NewNaiveDoctor(),
		nil,
	)
	assert.NotNil(t, s.(*searcher).peerSigner)
	assert.NotNil(t, s.(*searcher).orgSigner)
	assert.NotNil(t, s.(*searcher).finderCreator)
	assert.NotNil(t, s.(*searcher).rp)
	assert.NotNil(t, s.(*searcher).rec)
}

func TestSearcher_Search_ok(t *testing.T) {
	n, nClosestResponses := 32, uint(6)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, addressFinders, selfPeerIdxs, peerID := NewTestPeers(rng, n)
	orgID := ecid.NewPseudoRandom(rng)

	// create our searcher
	key := id.NewPseudoRandom(rng)

	for concurrency := uint(3); concurrency <= 3; concurrency++ {
		info := fmt.Sprintf("concurrency: %d", concurrency)
		//log.Printf("running: %s", info) // sometimes handy for debugging

		rec := &fixedRecorder{}
		searcher := NewTestSearcher(peersMap, addressFinders, rec)
		search := NewSearch(peerID, orgID, key, &Parameters{
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
		assert.True(t, search.Finished(), info)
		assert.True(t, search.FoundClosestPeers(), info)
		assert.False(t, search.Errored(), info)
		assert.False(t, search.Exhausted(), info)
		assert.Equal(t, 0, len(search.Result.Errored), info)
		assert.Equal(t, int(nClosestResponses), search.Result.Closest.Len(), info)
		assert.True(t, search.Result.Closest.Len() <= len(search.Result.Responded), info)

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
			assert.True(t, in, info)
		}

		// check responses got recorded
		assert.True(t, len(search.Result.Responded) <= rec.nSuccesses)
		assert.Zero(t, rec.nErrors)
	}
}

func TestSearcher_Search_queryErr(t *testing.T) {
	rec := &fixedRecorder{}
	searcherImpl, search, selfPeerIdxs, peers := newTestSearch(rec)
	seeds := NewTestSeeds(peers, selfPeerIdxs)

	// duplicate seeds so we cover branch of hitting errored peer more than once
	seeds = append(seeds, seeds[0])

	// all queries return errors
	searcherImpl.(*searcher).finderCreator = &TestFinderCreator{
		err: errors.New("some Create error"),
	}

	// do the search!
	err := searcherImpl.Search(search, seeds)

	// checks
	assert.Equal(t, ErrTooManyFindErrors, err)
	assert.True(t, search.Errored())    // since all of the queries return errors
	assert.False(t, search.Exhausted()) // since NMaxErrors < len(Unqueried)
	assert.True(t, search.Finished())
	assert.False(t, search.FoundClosestPeers())
	assert.Equal(t, int(search.Params.NMaxErrors+1), len(search.Result.Errored))
	assert.Equal(t, 0, search.Result.Closest.Len())
	assert.True(t, 0 < search.Result.Unqueried.Len())
	assert.Equal(t, 0, len(search.Result.Responded))
	assert.Equal(t, len(search.Result.Errored), rec.nErrors)
}

type errResponseProcessor struct{}

func (erp *errResponseProcessor) Process(rp *api.FindResponse, search *Search) error {
	return errors.New("some processing error")
}

func TestSearcher_Search_rpErr(t *testing.T) {
	rec := &fixedRecorder{}
	searcherImpl, search, selfPeerIdxs, peers := newTestSearch(rec)
	seeds := NewTestSeeds(peers, selfPeerIdxs)

	// mock some internal issue when processing responses
	searcherImpl.(*searcher).rp = &errResponseProcessor{}

	// do the search!
	err := searcherImpl.Search(search, seeds)

	// checks
	assert.NotNil(t, err)
	assert.NotNil(t, search.Result.FatalErr)
	assert.True(t, search.Errored()) // since we hit max number of allowable errors
	assert.False(t, search.Exhausted())
	assert.True(t, search.Finished())
	assert.False(t, search.FoundClosestPeers())
	assert.Equal(t, int(search.Params.NMaxErrors+1), len(search.Result.Errored))
	assert.Equal(t, 0, search.Result.Closest.Len())
	assert.True(t, 0 < search.Result.Unqueried.Len())
	assert.Equal(t, 0, len(search.Result.Responded))
	assert.Equal(t, len(search.Result.Errored), rec.nErrors)
}

func newTestSearch(rec comm.QueryRecorder) (Searcher, *Search, []int, []peer.Peer) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, peerConnectedAddrs, selfPeerIdxs, peerID := NewTestPeers(rng, n)
	orgID := ecid.NewPseudoRandom(rng)

	// create our searcher
	key := id.NewPseudoRandom(rng)
	searcher := NewTestSearcher(peersMap, peerConnectedAddrs, rec)

	search := NewSearch(peerID, orgID, key, &Parameters{
		NClosestResponses: nClosestResponses,
		NMaxErrors:        DefaultNMaxErrors,
		Concurrency:       uint(1),
		Timeout:           DefaultQueryTimeout,
	})
	return searcher, search, selfPeerIdxs, peers
}

func TestSearcher_query_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	search := NewSearch(peerID, orgID, key, &Parameters{})
	next := peer.NewTestPeer(rng, 0)
	s := &searcher{
		peerSigner: &client.TestNoOpSigner{},
		orgSigner:  &client.TestNoOpSigner{},
		finderCreator: &TestFinderCreator{
			finders: map[string]api.Finder{
				next.Address().String(): &fixedFinder{},
			},
		},
		rp: nil,
	}

	rp, err := s.query(next, search)
	assert.Nil(t, err)
	assert.NotNil(t, rp.Metadata.RequestId)
	assert.Nil(t, rp.Value)
}

func TestSearcher_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	search := NewSearch(peerID, orgID, key, &Parameters{Timeout: 1 * time.Second})
	next := peer.NewTestPeer(rng, 0)

	cases := []*searcher{
		// case 0
		{
			peerSigner:    &client.TestNoOpSigner{},
			orgSigner:     &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{err: errors.New("some create error")},
		},

		// case 1
		{
			peerSigner:    &client.TestErrSigner{},
			orgSigner:     &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{},
		},

		// case 2
		{
			peerSigner: &client.TestNoOpSigner{},
			orgSigner:  &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{
				err: errors.New("some Find error"),
			},
		},

		// case 3
		{
			peerSigner: &client.TestNoOpSigner{},
			orgSigner:  &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{
				finders: map[string]api.Finder{
					next.Address().String(): &fixedFinder{requestID: []byte{1, 2, 3, 4}},
				},
			},
		},
	}

	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp, err := c.query(next, search)
		assert.Nil(t, rp, info)
		assert.NotNil(t, err, info)
	}
}

func TestResponseProcessor_Process_Value(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := id.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer(), comm.NewNaiveDoctor())
	s := &Search{
		Result: NewInitialResult(key, NewDefaultParameters()),
	}

	// create response with the value
	value, _ := api.NewTestDocument(rng)
	response2 := &api.FindResponse{
		Peers: nil,
		Value: value,
	}

	// check that the result value is set
	prevUnqueriedLength := s.Result.Unqueried.Len()
	err := rp.Process(response2, s)
	assert.Nil(t, err)
	assert.Equal(t, prevUnqueriedLength, s.Result.Unqueried.Len())
	assert.Equal(t, value, s.Result.Value)
}

func TestResponseProcessor_Process_Addresses(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))

	// create placeholder api.PeerAddresses for our mocked api.FindPeers response
	nAddresses := 6
	peerAddresses := newPeerAddresses(rng, nAddresses)

	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer(), comm.NewNaiveDoctor())
	s := NewSearch(peerID, orgID, key, NewDefaultParameters())

	// create response or nAddresses and process it
	response := &api.FindResponse{
		Peers: peerAddresses,
		Value: nil,
	}
	err := rp.Process(response, s)
	assert.Nil(t, err)
	assert.Equal(t, nAddresses, s.Result.Unqueried.Len())
}

func TestResponseProcessor_Process_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := id.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer(), comm.NewNaiveDoctor())
	s := &Search{
		Result: NewInitialResult(key, NewDefaultParameters()),
	}

	// create a bad response with neither a value nor peer addresses
	response2 := &api.FindResponse{
		Peers: nil,
		Value: nil,
	}
	err := rp.Process(response2, s)
	assert.NotNil(t, err)
}

func TestAddPeers(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))

	// create placeholder api.PeerAddresses for our mocked api.FindPeers response
	nAddresses1 := 6
	peerAddresses1 := newPeerAddresses(rng, nAddresses1)

	key := id.NewPseudoRandom(rng)
	fromer := peer.NewFromer()
	allHealthyDoc := &fixedDoctor{healthy: true}
	allUnhealthyDoc := &fixedDoctor{healthy: false}
	queried := make(map[string]struct{})
	unqueried := NewClosestPeers(key, 9)

	// check that when peers are deemed unhealthy, they're not added
	AddPeers(queried, unqueried, allUnhealthyDoc, peerAddresses1, fromer)
	assert.Zero(t, unqueried.Len())

	// check that all peers go into the unqueried heap
	AddPeers(queried, unqueried, allHealthyDoc, peerAddresses1, fromer)
	assert.Equal(t, nAddresses1, unqueried.Len())

	// add same peers and check that the length of unqueried hasn't changed
	AddPeers(queried, unqueried, allHealthyDoc, peerAddresses1, fromer)
	assert.Equal(t, nAddresses1, unqueried.Len())

	// create new peers and add them to the queried map (as if we'd already heard from them)
	nAddresses2 := 3
	peerAddresses2 := newPeerAddresses(rng, nAddresses2)
	for _, pa := range peerAddresses2 {
		p := fromer.FromAPI(pa)
		queried[p.ID().String()] = struct{}{}
	}

	// check that adding these peers again has no effect
	AddPeers(queried, unqueried, allHealthyDoc, peerAddresses2, fromer)
	assert.Equal(t, nAddresses1, unqueried.Len())
	assert.Equal(t, nAddresses2, len(queried))
}

func newPeerAddresses(rng *rand.Rand, n int) []*api.PeerAddress {
	peerAddresses := make([]*api.PeerAddress, n)
	for i, p := range peer.NewTestPeers(rng, n) {
		peerAddresses[i] = p.ToAPI()
	}
	return peerAddresses
}

type fixedRecorder struct {
	nSuccesses int
	nErrors    int
}

func (f *fixedRecorder) Record(
	peerID id.ID, endpoint api.Endpoint, qt comm.QueryType, o comm.Outcome,
) {
	if o == comm.Success {
		f.nSuccesses++
	} else {
		f.nErrors++
	}
}

func (f *fixedRecorder) Get(peerID id.ID, endpoint api.Endpoint) comm.QueryOutcomes {
	panic("implement me")
}

func (f *fixedRecorder) CountPeers(endpoint api.Endpoint, qt comm.QueryType, known bool) int {
	panic("implement me")
}

type fixedDoctor struct {
	healthy bool
}

func (d *fixedDoctor) Healthy(peerID id.ID) bool {
	return d.healthy
}
