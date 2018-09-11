package comm

import (
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// Doctor determines whether a peer is currently healthy.
type Doctor interface {

	// Healthy indicates whether the peer is currently deemed healthy.
	Healthy(peerID id.ID) bool
}

// NewDefaultDoctor returns a default Doctor.
func NewDefaultDoctor() Doctor {
	return NewNaiveDoctor()
}

type naiveDoctor struct{}

// NewNaiveDoctor returns a Doctor that naively assumes all peers are healthy.
func NewNaiveDoctor() Doctor {
	return &naiveDoctor{}
}

func (d *naiveDoctor) Healthy(peerID id.ID) bool {
	return true
}

// NewResponseTimeDoctor returns a Doctor that assumes peers are health if their latest successful
// response is within a fixed window of the latest unsuccessful/errored response.
func NewResponseTimeDoctor(recorder QueryGetter) Doctor {
	return &responseTimeDoctor{
		recorder: recorder,
	}
}

type responseTimeDoctor struct {
	recorder QueryGetter
}

func (d *responseTimeDoctor) Healthy(peerID id.ID) bool {
	// Verify endpoint should most regularly be used, so just check that one for now
	verifyOutcomes := d.recorder.Get(peerID, api.Verify)
	latestErrTime := verifyOutcomes[Response][Error].Latest
	latestSuccessTime := verifyOutcomes[Response][Success].Latest

	if latestErrTime.IsZero() && latestSuccessTime.IsZero() {
		// fall back to Find if no Verifications
		verifyOutcomes = d.recorder.Get(peerID, api.Find)
		latestErrTime = verifyOutcomes[Response][Error].Latest
		latestSuccessTime = verifyOutcomes[Response][Success].Latest
	}

	// assume healthy if latest error time less than 5 mins after success time
	return latestErrTime.Before(latestSuccessTime.Add(5 * time.Minute))
}
