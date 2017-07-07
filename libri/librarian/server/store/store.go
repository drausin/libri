package store

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"go.uber.org/zap/zapcore"
)

const (
	// DefaultNReplicas is the numeber of
	DefaultNReplicas = uint(3)

	// DefaultNMaxErrors is the maximum number of errors tolerated during a search.
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the number of parallel store workers.
	DefaultConcurrency = uint(3)

	// DefaultQueryTimeout is the timeout for each query to a peer.
	DefaultQueryTimeout = 5 * time.Second

	logSearch      = "search"
	logNReplicas   = "n_replicas"
	logNMaxErrors  = "n_max_errors"
	logConcurrency = "concurrency"
	logTimeout     = "timeout"
	logNUnqueried  = "n_unqueried"
	logNResponded  = "n_responded"
	logErrors      = "errors"
	logFatalError  = "fatal_error"
	logResult      = "result"
	logParams      = "params"
	logStored      = "stored"
	logFoundValue  = "found_value"
	logExists      = "exists"
	logErrored     = "errored"
	logExhausted   = "exhausted"
	logFinished    = "finished"
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

func (p *Parameters) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddUint(logNReplicas, p.NReplicas)
	oe.AddUint(logNMaxErrors, p.NMaxErrors)
	oe.AddUint(logConcurrency, p.Concurrency)
	oe.AddDuration(logTimeout, p.Timeout)
	return nil
}

// Result holds the store's (intermediate) result: the number of peers that have successfully
// stored the value.
type Result struct {
	// Responded contains the peers that have successfully stored the value
	Responded []peer.Peer

	// Unqueried is a queue of peers to send store queries to
	Unqueried []peer.Peer

	// Search is search result from first part of store operation
	Search *search.Result

	// Errors is a list of errors encounters while querying peers
	Errors []error

	// FatalErr is the fatal error that occurred during the search
	FatalErr error
}

// NewInitialResult creates a new Result object from the final search result.
func NewInitialResult(sr *search.Result) *Result {
	return &Result{
		// send store queries to the closest peers from the search
		Unqueried: sr.Closest.Peers(),
		Responded: make([]peer.Peer, 0, sr.Closest.Len()),
		Search:    sr,
		Errors:    make([]error, 0),
	}
}

// NewFatalResult creates a new Result object with a fatal error.
func NewFatalResult(fatalErr error) *Result {
	return &Result{
		FatalErr: fatalErr,
	}
}

func (r *Result) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	if r == nil {
		return nil
	}
	oe.AddInt(logNUnqueried, len(r.Unqueried))
	oe.AddInt(logNResponded, len(r.Responded))
	oe.AddArray(logErrors, errArray(r.Errors))
	if r.FatalErr != nil {
		oe.AddString(logFatalError, r.FatalErr.Error())
	}
	return nil
}

type errArray []error

func (errs errArray) MarshalLogArray(arr zapcore.ArrayEncoder) error {
	for _, err := range errs {
		arr.AppendString(err.Error())
	}
	return nil
}

// Store contains things involved in storing a particular key/value pair.
type Store struct {
	// Request used when querying peers
	Request *api.StoreRequest // TODO (drausin) make this a getRequest function instead

	// Result of the store
	Result *Result

	// Search first part of store operation
	Search *search.Search

	// Params defining the store part of the operation
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
	updatedSearchParams := *searchParams // by value to avoid change original search params
	updatedSearchParams.NClosestResponses = storeParams.NReplicas + storeParams.NMaxErrors
	return &Store{
		Request: client.NewStoreRequest(peerID, key, value),
		Search:  search.NewSearch(peerID, key, &updatedSearchParams),
		Params:  storeParams,
	}
}

func (s *Store) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	if s == nil {
		return nil
	}
	oe.AddObject(logParams, s.Params)
	oe.AddObject(logSearch, s.Search)
	oe.AddObject(logResult, s.Result)
	if s.Result != nil {
		oe.AddBool(logFinished, s.Finished())
		oe.AddBool(logStored, s.Stored())
		oe.AddBool(logExists, s.Exists())
		oe.AddBool(logErrored, s.Errored())
		oe.AddBool(logExhausted, s.Exhausted())
	}
	return nil
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
	return len(s.Result.Errors) >= int(s.Params.NMaxErrors) || s.Result.FatalErr != nil
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
