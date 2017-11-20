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
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultSearcher(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s := NewDefaultSearcher(client.NewSigner(ecid.NewPseudoRandom(rng).Key()))
	assert.NotNil(t, s.(*searcher).signer)
	assert.NotNil(t, s.(*searcher).finderCreator)
	assert.NotNil(t, s.(*searcher).rp)
}

func TestSearcher_Search_ok(t *testing.T) {
	n, nClosestResponses := 32, uint(6)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := NewTestPeers(rng, n)

	// create our searcher
	key := id.NewPseudoRandom(rng)
	searcher := NewTestSearcher(peersMap)

	for concurrency := uint(3); concurrency <= 3; concurrency++ {
		info := fmt.Sprintf("concurrency: %d", concurrency)
		//log.Printf("running: %s", info) // sometimes handy for debugging

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

		if !search.FoundClosestPeers() {
			// very occasionally, this test will fail b/c we get one or more new unqueried peer(s)
			// closer than the farthest queried peer after the searcher has already declared the
			// search finished; this only very rarely happens in the wild, so just hack around it
			// here
			//
			// after removing closest concurrency-1 peers, everything should be back to
			// normal/finished
			for d := uint(0); d < concurrency-1; d++ {
				if search.Result.Unqueried.Len() > 0 {
					heap.Pop(search.Result.Unqueried)
				}
			}
		}

		// this is the "main" case we expect to get 97% of the time
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
	}
}

func TestSearcher_Search_queryErr(t *testing.T) {
	searcherImpl, search, selfPeerIdxs, peers := newTestSearch()
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
}

type errResponseProcessor struct{}

func (erp *errResponseProcessor) Process(rp *api.FindResponse, search *Search) error {
	return errors.New("some processing error")
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
	assert.True(t, search.Errored()) // since we hit max number of allowable errors
	assert.False(t, search.Exhausted())
	assert.True(t, search.Finished())
	assert.False(t, search.FoundClosestPeers())
	assert.Equal(t, int(search.Params.NMaxErrors+1), len(search.Result.Errored))
	assert.Equal(t, 0, search.Result.Closest.Len())
	assert.True(t, 0 < search.Result.Unqueried.Len())
	assert.Equal(t, 0, len(search.Result.Responded))
}

func newTestSearch() (Searcher, *Search, []int, []peer.Peer) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := NewTestPeers(rng, n)

	// create our searcher
	key := id.NewPseudoRandom(rng)
	searcher := NewTestSearcher(peersMap)

	search := NewSearch(selfID, key, &Parameters{
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
	search := NewSearch(peerID, key, &Parameters{})
	pConn := &peer.TestConnector{}
	s := &searcher{
		signer:        &client.TestNoOpSigner{},
		finderCreator: &TestFinderCreator{},
		rp:            nil,
	}

	rp, err := s.query(pConn, search)
	assert.Nil(t, err)
	assert.NotNil(t, rp.Metadata.RequestId)
	assert.Nil(t, rp.Value)
}

func TestSearcher_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	pConn := &peer.TestConnector{}
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	search := NewSearch(peerID, key, &Parameters{Timeout: 1 * time.Second})

	cases := []*searcher{
		// case 0
		{
			signer:        &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{err: errors.New("some create error")},
		},

		// case 1
		{
			signer:        &client.TestErrSigner{},
			finderCreator: &TestFinderCreator{},
		},

		// case 2
		{
			signer: &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{
				finder: &fixedFinder{err: errors.New("some Find error")},
			},
		},

		// case 3
		{
			signer: &client.TestNoOpSigner{},
			finderCreator: &TestFinderCreator{
				finder: &fixedFinder{requestID: []byte{1, 2, 3, 4}},
			},
		},
	}

	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp, err := c.query(pConn, search)
		assert.Nil(t, rp, info)
		assert.NotNil(t, err, info)
	}
}

func TestResponseProcessor_Process_Value(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := id.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
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

	key := id.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	s := &Search{
		Result: NewInitialResult(key, NewDefaultParameters()),
	}

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
	rp := NewResponseProcessor(peer.NewFromer())
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
	queried := make(map[string]struct{})
	unqueried := NewClosestPeers(key, 9)

	// check that all peers go into the unqueried heap
	AddPeers(queried, unqueried, peerAddresses1, fromer)
	assert.Equal(t, nAddresses1, unqueried.Len())

	// add same peers and check that the length of unqueried hasn't changed
	AddPeers(queried, unqueried, peerAddresses1, fromer)
	assert.Equal(t, nAddresses1, unqueried.Len())

	// create new peers and add them to the queried map (as if we'd already heard from them)
	nAddresses2 := 3
	peerAddresses2 := newPeerAddresses(rng, nAddresses2)
	for _, pa := range peerAddresses2 {
		p := fromer.FromAPI(pa)
		queried[p.ID().String()] = struct{}{}
	}

	// check that adding these peers again has no effect
	AddPeers(queried, unqueried, peerAddresses2, fromer)
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
