package goodwill

import (
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// Judge determines which peers should be favored over others.
type Judge interface {

	// Prefer indicates whether peer 1 should be preferred over peer 2 when deciding which to
	// query next.
	Prefer(peerID1, peerID2 id.ID) bool

	Trusted(peerID id.ID) bool

	Healthy(peerID id.ID) bool
}

type latestNaiveJudge struct {
	rec Recorder
}

// NewLatestPreferJudge returns a Judge using a strategy of preferring the peer with the most
// recent successful response. When both peers have responded within same minute, it prefers
// that peer with the fewer number of successful responses.
func NewLatestPreferJudge(rec Recorder) Judge {
	return &latestNaiveJudge{rec}
}

func (j *latestNaiveJudge) Prefer(peerID1, peerID2 id.ID) bool {
	// TODO (drausin) refactor this a bit
	rpSuccess1 := j.rec.Get(peerID1, api.Find)[Response][Success]
	rqSuccess1 := j.rec.Get(peerID1, api.Find)[Request][Success]
	rpSuccess2 := j.rec.Get(peerID2, api.Find)[Response][Success]
	rqSuccess2 := j.rec.Get(peerID2, api.Find)[Request][Success]

	latest1 := rqSuccess1.Latest
	if latest1.Before(rpSuccess1.Latest) {
		latest1 = rpSuccess1.Latest
	}
	latest2 := rqSuccess2.Latest
	if latest2.Before(rpSuccess2.Latest) {
		latest2 = rpSuccess2.Latest
	}
	diff1 := int64(rqSuccess1.Count) - int64(rpSuccess1.Count)
	if diff1 < 0 {
		diff1 = -diff1
	}
	diff2 := int64(rqSuccess2.Count) - int64(rpSuccess2.Count)
	if diff2 < 0 {
		diff2 = -diff2
	}

	// don't care about differences in latest response time within 5 minutes
	latestDiff := latest1.Sub(latest2)
	if latestDiff < 5*time.Minute && latestDiff > -5*time.Minute {
		// prefer peer1 if its balance is closer to 0, i.e., smaller than peer2's
		return diff1 < diff2
	}
	return latest1.After(latest2)
}

func (j *latestNaiveJudge) Trusted(peerID id.ID) bool {
	return true
}

func (j *latestNaiveJudge) Healthy(peerID id.ID) bool {
	return true
}
