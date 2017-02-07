package search

import (
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestSearch_FoundClosestPeers(t *testing.T) {
	target := cid.FromInt64(0) // target = 0 makes it easy to compute XOR distance manually
	nClosestResponses := uint(4)

	search := NewSearch(target, &Parameters{
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
	err = search.Result.unqueried.SafePush(peer.New(cid.FromInt64(5), "", nil))
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
	search := NewSearch(cid.FromInt64(0), NewParameters())

	// hasn't been found yet because result.value is still nil
	assert.False(t, search.FoundValue())

	// set the result
	search.Result.Value = cid.FromInt64(1).Bytes() // arbitrary, just needs to something
	assert.True(t, search.FoundValue())
}

func TestSearch_Errored(t *testing.T) {
	target := cid.FromInt64(0) // arbitrary but simple

	// no-error state
	search1 := NewSearch(target, NewParameters())
	search1.NErrors, search1.FatalErr = 0, nil
	assert.False(t, search1.Errored())

	// errored state b/c of too many errors
	search2 := NewSearch(target, NewParameters())
	search2.NErrors = search2.Params.NMaxErrors
	assert.True(t, search2.Errored())

	// errored state b/c of a fatal error
	search3 := NewSearch(target, NewParameters())
	search3.FatalErr = errors.New("test fatal error")
	assert.True(t, search3.Errored())
}

func TestSearch_Exhausted(t *testing.T) {
	target := cid.FromInt64(0) // arbitrary but simple

	// not exhausted b/c it has unqueried peers
	search1 := NewSearch(target, NewParameters())
	err := search1.Result.unqueried.SafePush(peer.New(cid.FromInt64(1), "", nil))
	assert.Nil(t, err)
	assert.False(t, search1.Exhausted())

	// exhausted b/c it doesn't have unqueried peers
	search2 := NewSearch(target, NewParameters())
	err = search2.Result.unqueried.SafePush(peer.New(cid.FromInt64(1), "", nil))
	assert.Nil(t, err)
	assert.False(t, search1.Exhausted())
}
