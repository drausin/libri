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

	for _, searchType := range []Type{Peers, Value} {
		// ClosestPeers() behaves the same for Peers and Value searches

		search := NewSearch(target, searchType, &Parameters{
			nClosestResponses: nClosestResponses,
		})

		// add closest peers to half the heap's capacity
		err := search.result.closest.SafePushMany([]peer.Peer{
			peer.New(cid.FromInt64(1), "", nil),
			peer.New(cid.FromInt64(2), "", nil),
		})
		assert.Nil(t, err)

		// haven't found closest peers b/c closest heap not at capacity
		assert.False(t, search.FoundClosestPeers())

		// add an unqueried peer farther than the farthest closest peer
		err = search.result.unqueried.SafePush(peer.New(cid.FromInt64(5), "", nil))
		assert.Nil(t, err)

		// still haven't found closest peers b/c closest heap not at capacity
		assert.False(t, search.FoundClosestPeers())

		// add two more closest peers, bringing closest heap to capacity
		err = search.result.closest.SafePushMany([]peer.Peer{
			peer.New(cid.FromInt64(3), "", nil),
			peer.New(cid.FromInt64(4), "", nil),
		})
		assert.Nil(t, err)

		// now that closest peers is at capacity, and it's max distance is less than the min
		// unqueried peers distance, search has found it's closest peers
		assert.True(t, search.FoundClosestPeers())
	}

}

func TestPeersSearch_FoundValue(t *testing.T) {
	search := NewSearch(cid.FromInt64(0), Peers, NewParameters())

	// Peers searches can never find the value
	assert.False(t, search.FoundValue())

	// technically this is possible in a Peers search, but it shouldn't do anything
	search.result.value = cid.FromInt64(1).Bytes() // arbitrary, just needs to something

	// Peers searches can never find the value
	assert.False(t, search.FoundValue())
}

func TestValueSearch_FoundValue(t *testing.T) {
	search := NewSearch(cid.FromInt64(0), Value, NewParameters())

	// hasn't been found yet because result.value is still nil
	assert.False(t, search.FoundValue())

	// set the result
	search.result.value = cid.FromInt64(1).Bytes() // arbitrary, just needs to something
	assert.True(t, search.FoundValue())
}

func TestPeersSearch_Missing(t *testing.T) {
	target := cid.FromInt64(0) // target = 0 makes it easy to compute XOR distance manually
	nClosestResponses := int64(4)

	search := NewSearch(target, Peers, &Parameters{nClosestResponses: uint(nClosestResponses)})

	// fill up closest peers and add another unqueried peer
	for c := int64(1); c <= nClosestResponses; c++ {
		err := search.result.closest.SafePush(peer.New(cid.FromInt64(c), "", nil))
		assert.Nil(t, err)
	}
	err := search.result.unqueried.SafePush(
		peer.New(cid.FromInt64(nClosestResponses+1), "", nil),
	)
	assert.Nil(t, err)

	// have found closest peers
	assert.True(t, search.FoundClosestPeers())

	// but since Peers searches never involve a value, they can't be missing
	assert.False(t, search.Missing())

	// even when we set the result, which should never happen in the wild
	search.result.value = cid.FromInt64(int64(1)).Bytes() // arbitrary
	assert.False(t, search.Missing())
}

func TestValueSearch_Missing(t *testing.T) {
	target := cid.FromInt64(0) // target = 0 makes it easy to compute XOR distance manually
	nClosestResponses := int64(4)

	search := NewSearch(target, Value, &Parameters{nClosestResponses: uint(nClosestResponses)})

	// fill up closest peers and add another unqueried peer
	for c := int64(1); c <= nClosestResponses; c++ {
		err := search.result.closest.SafePush(peer.New(cid.FromInt64(c), "", nil))
		assert.Nil(t, err)
	}
	err := search.result.unqueried.SafePush(
		peer.New(cid.FromInt64(nClosestResponses+1), "", nil),
	)
	assert.Nil(t, err)

	// have found closest peers
	assert.True(t, search.FoundClosestPeers())

	// even though we've found the closest peers, still missing the value
	assert.True(t, search.Missing())

	//// now, if we've found the value, it's no longer missing
	search.result.value = cid.FromInt64(int64(1)).Bytes() // arbitrary
	assert.False(t, search.Missing())
}

func TestSearch_Errored(t *testing.T) {
	target := cid.FromInt64(0) // arbitrary but simple

	for _, searchType := range []Type{Peers, Value} {
		// no-error state
		search1 := NewSearch(target, searchType, NewParameters())
		search1.nErrors, search1.fatalErr = 0, nil
		assert.False(t, search1.Errored())

		// errored state b/c of too many errors
		search2 := NewSearch(target, searchType, NewParameters())
		search2.nErrors = search2.params.nMaxErrors
		assert.True(t, search2.Errored())

		// errored state b/c of a fatal error
		search3 := NewSearch(target, searchType, NewParameters())
		search3.fatalErr = errors.New("test fatal error")
		assert.True(t, search3.Errored())
	}
}

func TestSearch_Exhausted(t *testing.T) {
	target := cid.FromInt64(0) // arbitrary but simple

	for _, searchType := range []Type{Peers, Value} {
		// not exhausted b/c it has unqueried peers
		search1 := NewSearch(target, searchType, NewParameters())
		err := search1.result.unqueried.SafePush(peer.New(cid.FromInt64(1), "", nil))
		assert.Nil(t, err)
		assert.False(t, search1.Exhausted())

		// exhausted b/c it doesn't have unqueried peers
		search2 := NewSearch(target, searchType, NewParameters())
		err = search2.result.unqueried.SafePush(peer.New(cid.FromInt64(1), "", nil))
		assert.Nil(t, err)
		assert.False(t, search1.Exhausted())
	}
}
