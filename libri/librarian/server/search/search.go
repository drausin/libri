package search

import (
	"sync"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/pkg/errors"
)

// Type indicates a type of search.
type Type int

const (
	// Peers searches are for peers close to a target.
	Peers Type = iota

	// Value searches are for a target value.
	Value
)

var (
	// DefaultNMaxErrors is the maximum number of errors tolerated during a search.
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the number of parallel search workers.
	DefaultConcurrency = uint(3)

	// DefaultQueryTimeout is the timeout for each query to a peer.
	DefaultQueryTimeout = 5 * time.Second
)

// Parameters defines the parameters of the search.
type Parameters struct {
	// required number of peers closest to the target we need to receive responses from
	nClosestResponses uint

	// maximum number of errors tolerated when querying peers during the search
	nMaxErrors uint

	// number of concurrent queries to use in search
	concurrency uint

	// timeout for queries to individual peers
	queryTimeout time.Duration
}

// NewParameters creates an instance with default parameters.
func NewParameters() *Parameters {
	return &Parameters{
		nClosestResponses: routing.DefaultMaxActivePeers,
		nMaxErrors:        DefaultNMaxErrors,
		concurrency:       DefaultConcurrency,
	}
}

// Result holds search's (intermediate) result: collections of peers and possibly the value.
type Result struct {

	// found value when searchType = Value, otherwise nil
	value []byte

	// heap of the responding peers found closest to the target
	closest FarthestPeers

	// heap of peers that were not yet queried before search ended
	unqueried ClosestPeers

	// map of all peers that responded during search
	responded map[string]peer.Peer
}

// NewInitialResult creates a new Result object for the beginning of a search.
func NewInitialResult(target cid.ID, params *Parameters) *Result {
	return &Result{
		value:     nil,
		closest:   newFarthestPeers(target, params.nClosestResponses),
		unqueried: newClosestPeers(target, params.nClosestResponses),
		responded: make(map[string]peer.Peer),
	}
}

// Search contains things involved in a search for a particular target.
type Search struct {
	// type of search
	searchType Type

	// ID search is looking for or close to
	target cid.ID

	// result of the search
	result *Result

	// parameters defining the search
	params *Parameters

	// number of errors encounters while querying peers
	nErrors uint

	// fatal error that occurred during the search
	fatalErr error

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

// NewSearch creates a new Search instance for a given target, search type, and search parameters.
func NewSearch(target cid.ID, searchType Type, params *Parameters) *Search {
	return &Search{
		searchType: searchType,
		target:     target,
		result:     NewInitialResult(target, params),
		params:     params,
		nErrors:    0,
	}
}

// FoundClosestPeers returns whether the search has found the closest peers to a target. This event
// occurs when it has received responses from the required number of peers, and the max distance of
// those peers to the target is less than the min distance of the peers we haven't queried yet.
func (s *Search) FoundClosestPeers() bool {
	if s.result.unqueried.Len() == 0 {
		// if we have no unqueried peers, just make sure closest peers heap is full
		return uint(s.result.closest.Len()) == s.params.nClosestResponses
	}

	// closest peers heap should be full and have a max distance less than the min unqueried
	// distance
	return uint(s.result.closest.Len()) == s.params.nClosestResponses &&
		s.result.closest.PeakDistance().Cmp(s.result.unqueried.PeakDistance()) <= 0
}

// FoundValue returns whether the search has found the target value. This can only happen with a
// Value search type.
func (s *Search) FoundValue() bool {
	return s.searchType == Value && s.result.value != nil
}

// Missing returns whether the search target is missing. This happens during a Value search that
// has found the peers closest to the target but not the target's value.
func (s *Search) Missing() bool {
	return s.searchType == Value && !s.FoundValue() && s.FoundClosestPeers()
}

// Errored returns whether the search has encountered too many errors when querying the peers.
func (s *Search) Errored() bool {
	return s.nErrors >= s.params.nMaxErrors || s.fatalErr != nil
}

// Exhausted returns whether the search has exhausted all unqueried peers close to the target.
func (s *Search) Exhausted() bool {
	return s.result.unqueried.Len() == 0
}

// Finished returns whether the search has finished, either because it has found the target or
// closest peers or errored or exhausted the list of peers to query. This operation is concurrency
// safe.
func (s *Search) Finished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.searchType == Value {
		return s.FoundValue() || s.Missing() || s.Errored() || s.Exhausted()

	}
	if s.searchType == Peers {
		return s.FoundClosestPeers() || s.Errored() || s.Exhausted()
	}

	panic(errors.New("should never get here"))
}
