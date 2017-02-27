package introduce

import (
	"time"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"sync"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
)

var (
	// DefaultTargetNumIntroductions is the default target number of peers to get introduced to.
	DefaultTargetNumIntroductions = uint(128)

	// DefaultNumPeersPerRequest is the default number of peers to ask for in each introduce
	// request.
	DefaultNumPeersPerRequest = uint(16)

	// DefaultNMaxErrors is the default number of errors tolerated when querying peers during
	// the introduction
	DefaultNMaxErrors = uint(3)

	// DefaultConcurrency is the number of parallel search workers.
	DefaultConcurrency = uint(3)

	// DefaultQueryTimeout is the timeout for each query to a peer.
	DefaultQueryTimeout = 5 * time.Second
)

type Parameters struct {
	// target number of peers to become introduced to
	TargetNumIntroductions uint

	// number of peers to ask for in each request
	NumPeersPerRequest uint

	// maximum number of errors tolerated when querying peers during the introduction
	NMaxErrors uint

	// number of concurrent queries to use in search
	Concurrency uint

	// timeout for queries to individual peers
	Timeout time.Duration
}

// NewDefaultParameters creates a new instance of default introduction parameters.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		TargetNumIntroductions: DefaultTargetNumIntroductions,
		NumPeersPerRequest: DefaultNumPeersPerRequest,
		NMaxErrors: DefaultNMaxErrors,
		Concurrency: DefaultConcurrency,
		Timeout: DefaultQueryTimeout,
	}
}

// Result holds an introduction's (intermediate) result.
type Result struct {

	// map of peers not yet queried
	Unqueried map[string]peer.Peer

	// map of all peers that that responded to introductions
	Responded map[string]peer.Peer

	// number of errors encountered while querying peers
	NErrors uint

	// fatal error that occurred during the search
	FatalErr error
}

// NewInitialResult creates a new Result for the beginning of an introduction.
func NewInitialResult() *Result {
	return &Result{
		Unqueried: make(map[string]peer.Peer),
		Responded: make(map[string]peer.Peer),
	}
}

// Introduction contains things involved in bootstrapping introductions.
type Introduction struct {
	// function to generate a new introduction request
	NewRequest func() *api.IntroduceRequest

	// result of the introduction
	Result     *Result

	// parameters defining the search
	Params     *Parameters

	// mutex used to synchronizes reads and writes to this instance
	mu         sync.Mutex
}

// NewIntroduction creates a new Introduction instance.
func NewIntroduction(selfID ecid.ID, apiSelf *api.PeerAddress, params *Parameters) *Introduction {
	return &Introduction{
		NewRequest: func() *api.IntroduceRequest {
			return api.NewIntroduceRequest(selfID, apiSelf, params.NumPeersPerRequest)
		},
		Result: NewInitialResult(),
		Params: params,
	}
}

func (i *Introduction) WrapLock(operation func()) {
	i.mu.Lock()
	defer i.mu.Unlock()
	operation()
}

// Errored returns whether the introduction has encountered too many errors when querying the peers.
func (i *Introduction) Errored() bool {
	return i.Result.NErrors >= i.Params.NMaxErrors || i.Result.FatalErr != nil
}

// Completed returns whether introductions have occurred with the target number of peers.
func (i *Introduction) ReachedTarget() bool {
	return uint(len(i.Result.Responded)) >= i.Params.TargetNumIntroductions
}

// Exhausted returns whether all of the possible peers have been queried.
func (s *Introduction) Exhausted() bool {
	return len(s.Result.Unqueried) == 0
}

// Finished returns whether the introduction has finished, either because it has reached the target
// number of peers, or has exhausted all the possible peers to query, or has encountered too many
// errors.
func (s *Introduction) Finished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ReachedTarget() || s.Errored() || s.Exhausted()
}