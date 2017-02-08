package store

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
)

var (
	// DefaultNMaxErrors is the maximum number of errors tolerated during a search.
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the number of parallel search workers.
	DefaultConcurrency = uint(3)

	// DefaultQueryTimeout is the timeout for each query to a peer.
	DefaultQueryTimeout = 5 * time.Second
)

// Parameters defines the parameters of the store.
type Parameters struct {
	// maximum number of errors tolerated when querying peers during the store
	NMaxErrors uint

	// number of concurrent queries to use in store
	Concurrency uint

	// timeout for queries to individual peers
	Timeout time.Duration
}

// NewParameters creates an instance with default parameters.
func NewParameters() *Parameters {
	return &Parameters{
		NMaxErrors:  DefaultNMaxErrors,
		Concurrency: DefaultConcurrency,
		Timeout:     DefaultQueryTimeout,
	}
}

// Result holds the store's (intermediate) result: the number of peers that have successfully
// stored the value.
type Result struct {
	// peers that have successfully stored the value
	Responded []peer.Peer

	// queue of peers to send store queries to
	Unqueried []peer.Peer

	// result of search used in first part of store operation
	Search *search.Result
}

// NewInitialResult creates a new Result object from the final search result.
func NewInitialResult(sr *search.Result) *Result {
	return &Result{
		// send store queries to the closest peers from the search
		Unqueried: sr.Closest.ToSlice(),
		Responded: make([]peer.Peer, 0, sr.Closest.Len()),
		Search:    sr,
	}
}

type Store struct {
	// request used when querying peers
	Request *api.StoreRequest

	// result of the store
	Result *Result

	// first part of store operation is the search
	Search *search.Search

	// parameters defining the store part of the operation
	Params *Parameters

	// number of errors encounters while querying peers
	NErrors uint

	// fatal error that occurred during the search
	FatalErr error

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

// NewSearch creates a new Store instance for a given target, search type, and search parameters.
func NewStore(search *search.Search, value []byte, params *Parameters) *Store {
	return &Store{
		Request: api.NewStoreRequest(search.Key, value),
		Search:  search,
		Params:  params,
		NErrors: 0,
	}
}

// Stored returns whether the store has stored sufficient replicas.
func (s *Store) Stored() bool {
	return uint(len(s.Result.Responded))+s.NErrors == s.Search.Params.NClosestResponses
}

// Errored returns whether the store has encountered too many errors when querying the peers.
func (s *Store) Errored() bool {
	return s.NErrors >= s.Params.NMaxErrors || s.FatalErr != nil
}

func (s *Store) Finished() bool {
	return s.Stored() || s.Errored()
}
