package goodwill

import (
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// Judge determines which peers should be favored over others.
type Judge interface {
	Preferer
	Truster
	Doctor
}

type latestNaiveJudge struct {
	naiveDoctor
	naiveTruster

	rec   Recorder
	finds Preferer
}

// NewLatestPreferJudge returns a Judge using a strategy of preferring the peer with the most
// recent successful response. When both peers have responded within same minute, it prefers
// that peer with the fewer number of successful responses.
func NewLatestPreferJudge(rec Recorder) Judge {
	return &latestNaiveJudge{
		naiveDoctor:  naiveDoctor{},
		naiveTruster: naiveTruster{},
		rec:          rec,
	}
}

func (j *latestNaiveJudge) Prefer(peerID1, peerID2 id.ID) bool {
	// don't care about differences in lastCalc response time within 5 minutes
	latest1, latest2 := j.getLatestInteraction(peerID1), j.getLatestInteraction(peerID2)
	latestDiff := latest1.Sub(latest2)
	if latestDiff < 5*time.Minute && latestDiff > -5*time.Minute {
		// prefer peer1 if its balance is closer to 0, i.e., smaller than peer2's
		return j.finds.Prefer(peerID1, peerID2)
	}
	return latest1.After(latest2)
}

func (j *latestNaiveJudge) getLatestInteraction(peerID id.ID) time.Time {
	rpLatest := j.rec.Get(peerID, api.All)[Response][Success].Latest
	rqLatest := j.rec.Get(peerID, api.All)[Request][Success].Latest
	if rpLatest.After(rqLatest) {
		return rpLatest
	}
	return rqLatest
}
