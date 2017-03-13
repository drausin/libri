package search

import (
	"testing"

	"math/rand"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultParameters(t *testing.T) {
	p := NewDefaultParameters()
	assert.NotZero(t, p.NClosestResponses)
	assert.NotZero(t, p.NMaxErrors)
	assert.NotZero(t, p.Concurrency)
	assert.NotZero(t, p.Timeout)
}

func TestSearch_FoundClosestPeers(t *testing.T) {
	// target = 0 makes it easy to compute XOR distance manually
	rng := rand.New(rand.NewSource(0))
	target, selfID := cid.FromInt64(0), ecid.NewPseudoRandom(rng)
	nClosestResponses := uint(4)

	search := NewSearch(selfID, target, &Parameters{
		NClosestResponses: nClosestResponses,
	})

	// add closest peers to half the heap's capacity
	err := search.Result.Closest.SafePushMany([]peer.Peer{
		peer.New(cid.FromInt64(1), "", nil),
		peer.New(cid.FromInt64(2), "", nil),
	})
	assert.Nil(t, err)

	// haven't found closest peers b/c closest heap not at capacity
	assert.False(t, search.FoundClosestPeers())

	// add an unqueried peer farther than the farthest closest peer
	err = search.Result.Unqueried.SafePush(peer.New(cid.FromInt64(5), "", nil))
	assert.Nil(t, err)

	// still haven't found closest peers b/c closest heap not at capacity
	assert.False(t, search.FoundClosestPeers())

	// add two more closest peers, bringing closest heap to capacity
	err = search.Result.Closest.SafePushMany([]peer.Peer{
		peer.New(cid.FromInt64(3), "", nil),
		peer.New(cid.FromInt64(4), "", nil),
	})
	assert.Nil(t, err)

	// now that closest peers is at capacity, and it's max distance is less than the min
	// unqueried peers distance, search has found it's closest peers
	assert.True(t, search.FoundClosestPeers())

}

func TestSearch_FoundValue(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	search := NewSearch(ecid.NewPseudoRandom(rng), cid.FromInt64(0), NewDefaultParameters())

	// hasn't been found yet because result.value is still nil
	assert.False(t, search.FoundValue())

	// set the result
	search.Result.Value, _ = api.NewTestDocument(rng)
	assert.True(t, search.FoundValue())
}

func TestSearch_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	target, selfID := cid.FromInt64(0), ecid.NewPseudoRandom(rng)

	// no-error state
	search1 := NewSearch(selfID, target, NewDefaultParameters())
	search1.Result.NErrors, search1.Result.FatalErr = 0, nil
	assert.False(t, search1.Errored())

	// errored state b/c of too many errors
	search2 := NewSearch(selfID, target, NewDefaultParameters())
	search2.Result.NErrors = search2.Params.NMaxErrors
	assert.True(t, search2.Errored())

	// errored state b/c of a fatal error
	search3 := NewSearch(selfID, target, NewDefaultParameters())
	search3.Result.FatalErr = errors.New("test fatal error")
	assert.True(t, search3.Errored())
}

func TestSearch_Exhausted(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	target, selfID := cid.FromInt64(0), ecid.NewPseudoRandom(rng)

	// not exhausted b/c it has unqueried peers
	search1 := NewSearch(selfID, target, NewDefaultParameters())
	err := search1.Result.Unqueried.SafePush(peer.New(cid.FromInt64(1), "", nil))
	assert.Nil(t, err)
	assert.False(t, search1.Exhausted())

	// exhausted b/c it doesn't have unqueried peers
	search2 := NewSearch(selfID, target, NewDefaultParameters())
	err = search2.Result.Unqueried.SafePush(peer.New(cid.FromInt64(1), "", nil))
	assert.Nil(t, err)
	assert.False(t, search1.Exhausted())
}
