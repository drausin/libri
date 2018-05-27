package search

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"go.uber.org/zap/zapcore"
)

const (
	// DefaultNClosestResponses is the default number of peers to find closest to the key.
	DefaultNClosestResponses = uint(6)

	// DefaultNMaxErrors is the default maximum number of errors tolerated during a search.
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the default number of parallel search workers.
	DefaultConcurrency = uint(1)

	// DefaultQueryTimeout is the timeout for each query to a peer.
	DefaultQueryTimeout = 3 * time.Second

	// logging keys
	logKey               = "key"
	logNClosestResponses = "n_closest_responses"
	logNMaxErrors        = "n_max_errors"
	logConcurrency       = "concurrency"
	logTimeout           = "timeout"
	logNClosest          = "n_closest"
	logNUnqueried        = "n_unqueried"
	logNResponded        = "n_responded"
	logErrors            = "errors"
	logFatalError        = "fatal_error"
	logResult            = "result"
	logParams            = "params"
	logFoundClosestPeers = "found_closest_peers"
	logFoundValue        = "found_value"
	logErrored           = "errored"
	logExhausted         = "exhausted"
	logFinished          = "finished"
)

// Parameters defines the parameters of the search.
type Parameters struct {
	// NClosestResponses is the required number of peers closest to the key we need to receive
	// responses from
	NClosestResponses uint

	// NMaxErrors is the maximum number of errors tolerated when querying peers during the search
	NMaxErrors uint

	// Concurrency is the number of concurrent queries to use in search
	Concurrency uint

	// Timeout for queries to individual peers
	Timeout time.Duration
}

// NewDefaultParameters creates an instance with default parameters.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		NClosestResponses: DefaultNClosestResponses,
		NMaxErrors:        DefaultNMaxErrors,
		Concurrency:       DefaultConcurrency,
		Timeout:           DefaultQueryTimeout,
	}
}

// MarshalLogObject converts the Parameters into an object (which will become json) for logging.
func (p *Parameters) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddUint(logNClosestResponses, p.NClosestResponses)
	oe.AddUint(logNMaxErrors, p.NMaxErrors)
	oe.AddUint(logConcurrency, p.Concurrency)
	oe.AddDuration(logTimeout, p.Timeout)
	return nil
}

// Result holds search's (intermediate) result: collections of peers and possibly the value.
type Result struct {
	// Value found when looking for one, otherwise nil
	Value *api.Document

	// Closest is a heap of the responding peers found closest to the target
	Closest FarthestPeers

	// Unqueried is a heap of peers that were not yet queried
	Unqueried ClosestPeers

	// Queried is a set of all peers (keyed by peer.ID().String()) that have been queried (but
	// haven't yet necessarily responded or errored)
	Queried map[string]struct{}

	// Responded is a map of all peers that responded during search
	Responded map[string]peer.Peer

	// Errored contains the errors received by each peer (via string representation of peer ID)
	Errored map[string]error

	// FatalErr is a fatal error that occurred during the search
	FatalErr error
}

// NewInitialResult creates a new Result object for the beginning of a search.
func NewInitialResult(key id.ID, params *Parameters) *Result {
	return &Result{
		Value:     nil,
		Closest:   NewFarthestPeers(key, params.NClosestResponses),
		Unqueried: NewClosestPeers(key, params.NClosestResponses*params.Concurrency),
		Queried:   make(map[string]struct{}),
		Responded: make(map[string]peer.Peer),
		Errored:   make(map[string]error),
	}
}

// MarshalLogObject converts the Result into an object (which will become json) for logging.
func (r *Result) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddInt(logNClosest, r.Closest.Len())
	oe.AddInt(logNUnqueried, r.Unqueried.Len())
	oe.AddInt(logNResponded, len(r.Responded))
	errors.MaybePanic(oe.AddArray(logErrors, clogging.ToErrArray(r.Errored)))
	if r.FatalErr != nil {
		oe.AddString(logFatalError, r.FatalErr.Error())
	}
	return nil
}

// Search contains things involved in a search for a particular target.
type Search struct {
	// ID search is looking for or close to
	Key id.ID

	// CreatRq creates new Find requests
	CreatRq func() *api.FindRequest

	// result of the search
	Result *Result

	// parameters defining the search
	Params *Parameters

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

// NewSearch creates a new Search instance for a given target, search type, and search parameters.
func NewSearch(selfID ecid.ID, key id.ID, params *Parameters) *Search {
	createRq := func() *api.FindRequest {
		return client.NewFindRequest(selfID, key, params.NClosestResponses)
	}
	return &Search{
		Key:     key,
		CreatRq: createRq,
		Result:  NewInitialResult(key, params),
		Params:  params,
	}
}

// MarshalLogObject converts the Search into an object (which will become json) for logging.
func (s *Search) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString(logKey, id.Hex(s.Key.Bytes()))
	errors.MaybePanic(oe.AddObject(logParams, s.Params))
	errors.MaybePanic(oe.AddObject(logResult, s.Result))
	oe.AddBool(logFinished, s.Finished())
	oe.AddBool(logFoundClosestPeers, s.FoundClosestPeers())
	oe.AddBool(logFoundValue, s.FoundValue())
	oe.AddBool(logErrored, s.Errored())
	oe.AddBool(logExhausted, s.Exhausted())
	return nil
}

// FoundClosestPeers returns whether the search has found the closest peers to a target. This event
// occurs when it has received responses from the required number of peers, and the max distance of
// those peers to the target is less than the min distance of the peers we haven't queried yet.
func (s *Search) FoundClosestPeers() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if uint(s.Result.Closest.Len()) < s.Params.NClosestResponses {
		return false
	}
	if s.Result.Unqueried.Len() == 0 {
		return true
	}

	// closest peers heap should have a max distance less than the min unqueried distance
	return s.Result.Closest.PeakDistance().Cmp(s.Result.Unqueried.PeakDistance()) <= 0
}

// FoundValue returns whether the search has found the target value.
func (s *Search) FoundValue() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Result.Value != nil
}

// Errored returns whether the search has encountered too many errors when querying the peers.
func (s *Search) Errored() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint(len(s.Result.Errored)) > s.Params.NMaxErrors || s.Result.FatalErr != nil
}

// Exhausted returns whether the search has exhausted all unqueried peers close to the target.
func (s *Search) Exhausted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Result.Unqueried.Len() == 0
}

// Finished returns whether the search has finished, either because it has found the target or
// closest peers or errored or exhausted the list of peers to query. This operation is concurrency
// safe.
func (s *Search) Finished() bool {
	return s.FoundValue() || s.FoundClosestPeers() || s.Errored()
}

// AddQueried adds a peer to the queried set.
func (s *Search) AddQueried(p peer.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Result.Queried[p.ID().String()] = struct{}{}
}

func (s *Search) wrapLock(operation func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation()
}
