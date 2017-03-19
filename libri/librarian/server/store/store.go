package store

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
)

var (
	// DefaultNMaxErrors is the maximum number of errors tolerated during a search.
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the number of parallel store workers.
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

// NewDefaultParameters creates an instance with default parameters.
func NewDefaultParameters() *Parameters {
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

	// number of errors encounters while querying peers
	NErrors uint

	// fatal error that occurred during the search
	FatalErr error
}

// NewInitialResult creates a new Result object from the final search result.
func NewInitialResult(sr *search.Result) *Result {
	return &Result{
		// send store queries to the closest peers from the search
		Unqueried: sr.Closest.Peers(),
		Responded: make([]peer.Peer, 0, sr.Closest.Len()),
		Search:    sr,
		NErrors:   0,
	}
}

// Store contains things involved in storing a particular key/value pair.
type Store struct {
	// request used when querying peers
	Request *api.StoreRequest

	// result of the store
	Result *Result

	// first part of store operation is the search
	Search *search.Search

	// parameters defining the store part of the operation
	Params *Parameters

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

// NewStore creates a new Store instance for a given target, search type, and search parameters.
func NewStore(
	peerID ecid.ID, search *search.Search, value *api.Document, params *Parameters,
) *Store {
	return &Store{
		Request: client.NewStoreRequest(peerID, search.Key, value),
		Search:  search,
		Params:  params,
	}
}

// Stored returns whether the store has stored sufficient replicas.
func (s *Store) Stored() bool {
	return uint(len(s.Result.Responded))+s.Result.NErrors == s.Search.Params.NClosestResponses
}

// Exists returns whether the value already exists (and the search has found it).
func (s *Store) Exists() bool {
	return s.Result.Search.Value != nil
}

// Errored returns whether the store has encountered too many errors when querying the peers.
func (s *Store) Errored() bool {
	return s.Result.NErrors >= s.Params.NMaxErrors || s.Result.FatalErr != nil
}

// Exhausted returns whether the store has exhausted all peers to store the value in.
func (s *Store) Exhausted() bool {
	return len(s.Result.Unqueried) == 0
}

// Finished returns whether the store operation has finished.
func (s *Store) Finished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Stored() || s.Errored() || s.Exists()
}

func (s *Store) moreUnqueried() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Result.Unqueried) > 0
}

func (s *Store) wrapLock(operation func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation()
}
