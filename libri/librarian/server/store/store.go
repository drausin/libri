package store

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/common/id"
)

// TODO (drausin) const?
var (
	// DefaultNReplicas is the numeber of
	DefaultNReplicas = uint(3)

	// DefaultNMaxErrors is the maximum number of errors tolerated during a search.
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the number of parallel store workers.
	DefaultConcurrency = uint(3)

	// DefaultQueryTimeout is the timeout for each query to a peer.
	DefaultQueryTimeout = 5 * time.Second
)

// Parameters defines the parameters of the store.
type Parameters struct {
	// NReplicas is the number of replicas to store
	NReplicas uint

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
		NReplicas:   DefaultNReplicas,
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

// NewFatalResult creates a new Result object with a fatal error.
func NewFatalResult(fatalErr error) *Result {
	return &Result {
		FatalErr: fatalErr,
	}
}

// Store contains things involved in storing a particular key/value pair.
type Store struct {
	// request used when querying peers
	Request *api.StoreRequest // TODO (drausin) make this a getRequest function instead

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
	peerID ecid.ID,
	key id.ID,
	value *api.Document,
	searchParams *search.Parameters,
	storeParams *Parameters,
) *Store {
	// if store has NMaxErrors, we still want to be able to store NReplicas with remainder of
	// closest peers found during search
	updatedSearchParams := *searchParams  // by value to avoid change original search params
	updatedSearchParams.NClosestResponses = storeParams.NReplicas + storeParams.NMaxErrors
	return &Store{
		Request: client.NewStoreRequest(peerID, key, value),
		Search:  search.NewSearch(peerID, key, &updatedSearchParams),
		Params:  storeParams,
	}
}

// Stored returns whether the store has stored sufficient replicas.
func (s *Store) Stored() bool {
	return uint(len(s.Result.Responded)) >= s.Params.NReplicas
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
	return s.Stored() || s.Errored() || s.Exists() || s.Exhausted()
}

func (s *Store) moreUnqueried() bool {
	return len(s.Result.Unqueried) > 0
}

func (s *Store) safeMoreUnqueried() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moreUnqueried()
}

func (s *Store) wrapLock(operation func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation()
}
