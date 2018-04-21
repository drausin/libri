package goodwill

import (
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// PreferJudge determines which peers should be favored over others.
type PreferJudge interface {

	// Prefer indicates whether peer 1 should be preferred over peer 2 when deciding which to
	// query next.
	Prefer(peerID1, peerID2 id.ID) bool
}

type latestPreferJudge struct {
	rec Recorder
}

// NewLatestPreferJudge returns a PreferJudge using a strategy of preferring the peer with the most
// recent successful response. When both peers have responded within same minute, it prefers
// that peer with the fewer number of successful responses.
func NewLatestPreferJudge(rec Recorder) PreferJudge {
	return &latestPreferJudge{rec}
}

func (j *latestPreferJudge) Prefer(peerID1, peerID2 id.ID) bool {
	success1 := j.rec.Get(peerID1, api.All)[Response][Success]
	error1 := j.rec.Get(peerID1, api.All)[Response][Error]
	success2 := j.rec.Get(peerID2, api.All)[Response][Success]
	error2 := j.rec.Get(peerID1, api.All)[Response][Error]

	// don't care about differences in latest response time within a minute
	if success1.Latest.Round(time.Minute) == success2.Latest.Round(time.Minute) {
		// prefer 1 over 2 if we've made fewer queries to it, so we can attempt to
		// balance queries across peers
		return success1.Count+error1.Count < success2.Count+error2.Count
	}
	return success1.Latest.After(success2.Latest)
}
