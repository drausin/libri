package verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"go.uber.org/zap/zapcore"
	"sync"
	"time"
)

const (
	logKey               = "key"
	logNReplicas         = "n_replicas"
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
	logUnderReplicated   = "under_replicated"
	logFullyReplicated   = "fully_replicated"
	logErrored           = "errored"
	logExhausted         = "exhausted"
	logFinished          = "finished"
)

// Parameters defines the parameters of the verify.
type Parameters struct {
	// NReplicas is the required number of replicas (in addition to the self peer's replica) of a
	// document to consider it full verified.
	NReplicas uint

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

// NewDefaultParameters returns a default Verify parameters instance.
func NewDefaultParameters() *Parameters {
	nReplicas := store.DefaultNReplicas - 1 // since self already has a replica
	return &Parameters{
		NReplicas:         nReplicas,
		NClosestResponses: nReplicas + store.DefaultNMaxErrors,
		NMaxErrors:        search.DefaultNMaxErrors,
		Concurrency:       search.DefaultConcurrency,
		Timeout:           search.DefaultQueryTimeout,
	}
}

// MarshalLogObject converts the Parameters into an object (which will become json) for logging.
func (p *Parameters) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddUint(logNReplicas, p.NReplicas)
	oe.AddUint(logNClosestResponses, p.NClosestResponses)
	oe.AddUint(logNMaxErrors, p.NMaxErrors)
	oe.AddUint(logConcurrency, p.Concurrency)
	oe.AddDuration(logTimeout, p.Timeout)
	return nil
}

// Result holds verify's (intermediate) result during and after execution.
type Result struct {
	// Replicas gives the peers with verified replicas.
	Replicas []peer.Peer

	// Closest is a heap of the responding peers without a replica found closest to the key
	Closest search.FarthestPeers

	// Unqueried is a heap of peers that were not yet queried
	Unqueried search.ClosestPeers

	// Responded is a map of all peers that responded during verification
	Responded map[string]peer.Peer

	// Errored contains the errors received by each peer (via string representation of peer ID)
	Errored map[string]error

	// FatalErr is a fatal error that occurred during the search
	FatalErr error
}

// NewInitialResult creates a new Result object for the beginning of a search.
func NewInitialResult(key id.ID, params *Parameters) *Result {
	return &Result{
		Replicas:  make([]peer.Peer, 0),
		Closest:   search.NewFarthestPeers(key, params.NClosestResponses),
		Unqueried: search.NewClosestPeers(key, params.NClosestResponses*params.Concurrency),
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

// Verify contains things involved in a verification of a document.
type Verify struct {
	// Key of document to verify
	Key id.ID

	Value []byte

	ExpectedMAC []byte

	// RequestCreator creates new Verify requests
	RequestCreator func() *api.VerifyRequest

	// Result contains the verification result
	Result *Result

	// Params of the verification
	Params *Parameters

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

// NewVerify creates a new Verify instance for the given key with the given macKey, expected mac
// value, and params.
func NewVerify(selfID ecid.ID, key id.ID, value, macKey []byte, params *Parameters) *Verify {
	rqCreator := func() *api.VerifyRequest {
		return client.NewVerifyRequest(selfID, key, macKey, params.NClosestResponses)
	}

	// get the expected mac
	macer := hmac.New(sha256.New, macKey)
	_, err := macer.Write(value)
	errors.MaybePanic(err) // should never happen b/c sha256.Write always returns nil error
	mac := macer.Sum(nil)

	return &Verify{
		Key:            key,
		Value:          value,
		ExpectedMAC:    mac,
		RequestCreator: rqCreator,
		Result:         NewInitialResult(key, params),
		Params:         params,
	}
}

// MarshalLogObject converts the Search into an object (which will become json) for logging.
func (v *Verify) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString(logKey, id.Hex(v.Key.Bytes()))
	errors.MaybePanic(oe.AddObject(logParams, v.Params))
	errors.MaybePanic(oe.AddObject(logResult, v.Result))
	oe.AddBool(logFinished, v.Finished())
	oe.AddBool(logUnderReplicated, v.UnderReplicated())
	oe.AddBool(logFullyReplicated, v.FullyReplicated())
	oe.AddBool(logErrored, v.Errored())
	oe.AddBool(logExhausted, v.Exhausted())
	return nil
}

// UnderReplicated returns whether some (or no) replicas but all the closest peers were found.
func (v *Verify) UnderReplicated() bool {
	if v.FullyReplicated() {
		// if we've found all the replicas, no need to return that we've also found the closest
		// peers
		return false
	}

	if v.Result.Unqueried.Len() == 0 {
		// if we have no unqueried peers, just make sure closest peers heap is full
		return uint(v.Result.Closest.Len()) >= v.Params.NClosestResponses
	}

	// number of replicas + closest peers should be greater or equal to desired number of closest
	// responses, and the max closest peers distance should be smaller than the min unqueried peers
	// distance
	return uint(len(v.Result.Replicas)+v.Result.Closest.Len()) >= v.Params.NClosestResponses &&
		v.Result.Closest.PeakDistance().Cmp(v.Result.Unqueried.PeakDistance()) <= 0
}

// FullyReplicated returns whether sufficient replicas were found.
func (v *Verify) FullyReplicated() bool {
	return uint(len(v.Result.Replicas)) >= v.Params.NReplicas
}

// Errored returns whether the verify has encountered too many errors when querying the peers.
func (v *Verify) Errored() bool {
	return uint(len(v.Result.Errored)) > v.Params.NMaxErrors || v.Result.FatalErr != nil
}

// Exhausted returns whether the verify has exhausted all unqueried peers close to the key.
func (v *Verify) Exhausted() bool {
	return v.Result.Unqueried.Len() == 0
}

// Finished returns whether the search has finished, either because it has found the target or
// closest peers or errored or exhausted the list of peers to query. This operation is concurrency
// safe.
func (v *Verify) Finished() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.FullyReplicated() || v.UnderReplicated() || v.Errored() || v.Exhausted()
}

func (v *Verify) wrapLock(operation func()) {
	v.mu.Lock()
	defer v.mu.Unlock()
	operation()
}
