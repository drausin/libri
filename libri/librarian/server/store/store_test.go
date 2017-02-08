package store

import (
	"testing"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	cid "github.com/drausin/libri/libri/common/id"
	"math/rand"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/pkg/errors"
)

func TestStore_Stored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes()  // use ID for convenience, but could be anything

	// create search with result of closest peers
	nClosestResponse := uint(4)
	search := ssearch.NewSearch(key, &ssearch.Parameters{
		NClosestResponses: nClosestResponse,
	})

	closest := make([]peer.Peer, 4)
	for i := uint(0); i < nClosestResponse; i++ {
		closest[i] = peer.New(cid.FromInt64(int64(i) + 1), "", nil)
	}
	err := search.Result.Closest.SafePushMany(closest)
	assert.Nil(t, err)

	// create store with search
	store := NewStore(search, value, &Parameters{
		NMaxErrors: 3,
	})
	store.Result = NewInitialResult(search.Result)

	// not stored yet b/c have no peers that have responded to store query
	assert.False(t, store.Stored())
	assert.False(t, store.Finished())

	// add a few errors, still not stored
	store.NErrors += 2
	assert.False(t, store.Stored())
	assert.False(t, store.Finished())

	// once we receive responses from enough peers, it's stored
	store.Result.Responded = append(store.Result.Responded, closest[:2]...)
	assert.True(t, store.Stored())
	assert.True(t, store.Finished())
}

func TestStore_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := cid.NewPseudoRandom(rng)

	// create search with result of closest peers
	nClosestResponse := uint(4)
	search := ssearch.NewSearch(key, &ssearch.Parameters{
		NClosestResponses: nClosestResponse,
	})

	// create new store
	s := &Store{
		NErrors: 0,
		Params: &Parameters{NMaxErrors: 3},
		Result: &Result{Responded: make([]peer.Peer, 0)},
		Search: search,
	}

	// haven't received any errors yet
	assert.False(t, s.Errored())
	assert.False(t, s.Finished())

	// add errors, but not too many
	s.NErrors += 2

	// still within acceptable range of errors
	assert.False(t, s.Errored())
	assert.False(t, s.Finished())

	// push over the edge
	s.NErrors++
	assert.True(t, s.Errored())
	assert.True(t, s.Finished())

	// or, if we receive a fatal error
	s.NErrors = 0
	s.FatalErr = errors.New("some fatal error")
	assert.True(t, s.Errored())
	assert.True(t, s.Finished())
}

