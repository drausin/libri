package search

import (
	"sync"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
)

// Type indicates a type of search.
type Type int

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
	// required number of peers closest to the key we need to receive responses from
	NClosestResponses uint

	// maximum number of errors tolerated when querying peers during the search
	NMaxErrors uint

	// number of concurrent queries to use in search
	Concurrency uint

	// timeout for queries to individual peers
	Timeout time.Duration
}

// NewParameters creates an instance with default parameters.
func NewParameters() *Parameters {
	return &Parameters{
		NClosestResponses: routing.DefaultMaxActivePeers,
		NMaxErrors:        DefaultNMaxErrors,
		Concurrency:       DefaultConcurrency,
		Timeout:           DefaultQueryTimeout,
	}
}

// Result holds search's (intermediate) result: collections of peers and possibly the value.
type Result struct {
	// found value when searchType = Value, otherwise nil
	Value []byte

	// heap of the responding peers found closest to the target
	Closest FarthestPeers

	// heap of peers that were not yet queried before search ended
	Unqueried ClosestPeers

	// map of all peers that responded during search
	Responded map[string]peer.Peer

	// number of errors encounters while querying peers
	NErrors uint

	// fatal error that occurred during the search
	FatalErr error
}

// NewInitialResult creates a new Result object for the beginning of a search.
func NewInitialResult(key cid.ID, params *Parameters) *Result {
	return &Result{
		Value:     nil,
		Closest:   newFarthestPeers(key, params.NClosestResponses),
		Unqueried: newClosestPeers(key, params.NClosestResponses),
		Responded: make(map[string]peer.Peer),
		NErrors:   0,
	}
}

// Search contains things involved in a search for a particular target.
type Search struct {
	// ID search is looking for or close to
	Key cid.ID

	// request used when querying peers
	Request *api.FindRequest

	// result of the search
	Result *Result

	// parameters defining the search
	Params *Parameters

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

// NewSearch creates a new Search instance for a given target, search type, and search parameters.
func NewSearch(selfID ecid.ID, key cid.ID, params *Parameters) *Search {
	return &Search{
		Key:     key,
		Request: api.NewFindRequest(selfID, key, params.NClosestResponses),
		Result:  NewInitialResult(key, params),
		Params:  params,
	}
}

// FoundClosestPeers returns whether the search has found the closest peers to a target. This event
// occurs when it has received responses from the required number of peers, and the max distance of
// those peers to the target is less than the min distance of the peers we haven't queried yet.
func (s *Search) FoundClosestPeers() bool {
	if s.Result.Unqueried.Len() == 0 {
		// if we have no unqueried peers, just make sure closest peers heap is full
		return uint(s.Result.Closest.Len()) == s.Params.NClosestResponses
	}

	// closest peers heap should be full and have a max distance less than the min unqueried
	// distance
	return uint(s.Result.Closest.Len()) == s.Params.NClosestResponses &&
		s.Result.Closest.PeakDistance().Cmp(s.Result.Unqueried.PeakDistance()) <= 0
}

// FoundValue returns whether the search has found the target value.
func (s *Search) FoundValue() bool {
	return s.Result.Value != nil
}

// Errored returns whether the search has encountered too many errors when querying the peers.
func (s *Search) Errored() bool {
	return s.Result.NErrors >= s.Params.NMaxErrors || s.Result.FatalErr != nil
}

// Exhausted returns whether the search has exhausted all unqueried peers close to the target.
func (s *Search) Exhausted() bool {
	return s.Result.Unqueried.Len() == 0
}

// Finished returns whether the search has finished, either because it has found the target or
// closest peers or errored or exhausted the list of peers to query. This operation is concurrency
// safe.
func (s *Search) Finished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.FoundValue() || s.FoundClosestPeers() || s.Errored() || s.Exhausted()
}
