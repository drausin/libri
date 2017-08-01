package verify

import (
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"sync"
	"github.com/drausin/libri/libri/common/ecid"
)

type Parameters struct {
	search.Parameters

	NReplicas uint
}

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

type Verify struct {
	// Key of document to verify
	Key id.ID

	// RequestCreator creates new Verify requests
	RequestCreator func() *api.VerifyRequest

	// Result contains the verification result
	Result *Result

	// Params of the verification
	Params *Parameters

	// mutex used to synchronizes reads and writes to this instance
	mu sync.Mutex
}

func NewVerify(selfID ecid.ID, key id.ID, params *Parameters) *Verify {
	// TODO
}

// PartiallyReplicated returns whether some (or no) replicas but all the closest peers were found.
func (v *Verify) PartiallyReplicated() bool {
	if v.Result.Unqueried.Len() == 0 {
		// if we have no unqueried peers, just make sure closest peers heap is full
		return uint(v.Result.Closest.Len()) >= v.Params.NClosestResponses
	}

	if v.FullyReplicated() {
		// if we've found all the replicas, no need to return that we've also found the closest
		// peers
		return false
	}

	// number of replicas + closest peers should be greater or equal to desired number of closest
	// responses, and the max closest peers distance should be smaller than the min unqueried peers
	// distance
	return uint(len(v.Result.Replicas) + v.Result.Closest.Len()) >= v.Params.NClosestResponses &&
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
	return v.FullyReplicated() || v.PartiallyReplicated() || v.Errored() || v.Exhausted()
}
