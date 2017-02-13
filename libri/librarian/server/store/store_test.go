package store

import (
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestStore_Stored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key := cid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything

	// create search with result of closest peers
	nClosestResponse := uint(4)
	search := ssearch.NewSearch(peerID, key, &ssearch.Parameters{
		NClosestResponses: nClosestResponse,
	})

	closest := make([]peer.Peer, 4)
	for i := uint(0); i < nClosestResponse; i++ {
		closest[i] = peer.New(cid.FromInt64(int64(i)+1), "", nil)
	}
	err := search.Result.Closest.SafePushMany(closest)
	assert.Nil(t, err)

	// create store with search
	store := NewStore(peerID, search, value, &Parameters{
		NMaxErrors: 3,
	})
	store.Result = NewInitialResult(search.Result)

	// not stored yet b/c have no peers that have responded to store query
	assert.False(t, store.Stored())
	assert.False(t, store.Finished())

	// add a few errors, still not stored
	store.Result.NErrors += 2
	assert.False(t, store.Stored())
	assert.False(t, store.Finished())

	// once we receive responses from enough peers, it's stored
	store.Result.Responded = append(store.Result.Responded, closest[:2]...)
	assert.True(t, store.Stored())
	assert.True(t, store.Finished())
}

func TestStore_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key := cid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng)

	// create search with result of closest peers
	nClosestResponse := uint(4)
	search := ssearch.NewSearch(peerID, key, &ssearch.Parameters{
		NClosestResponses: nClosestResponse,
	})

	// create new store
	searchResult := ssearch.NewInitialResult(key, &ssearch.Parameters{
		NMaxErrors: 3,
	})
	s := &Store{
		Params: &Parameters{NMaxErrors: 3},
		Result: NewInitialResult(searchResult),
		Search: search,
	}

	// haven't received any errors yet
	assert.False(t, s.Errored())
	assert.False(t, s.Finished())

	// add errors, but not too many
	s.Result.NErrors += 2

	// still within acceptable range of errors
	assert.False(t, s.Errored())
	assert.False(t, s.Finished())

	// push over the edge
	s.Result.NErrors++
	assert.True(t, s.Errored())
	assert.True(t, s.Finished())

	// or, if we receive a fatal error
	s.Result.NErrors = 0
	s.Result.FatalErr = errors.New("some fatal error")
	assert.True(t, s.Errored())
	assert.True(t, s.Finished())
}
