package peer

import (
	"time"

	"github.com/drausin/libri/libri/librarian/server/storage"
)

// ResponseRecorder tracks statistics associated with the responses of a peer.
type ResponseRecorder interface {

	// Success records a successful response from the peer.
	Success()

	// Error records an unsuccessful or error-laden response from the peer.
	Error()

	// ToStored creates a storage.ResponseStats.
	ToStored() *storage.Responses
}

// ResponseStats describes metrics associated with a peer's communication history.
type responseRecorder struct {
	// earliest response time from the peer
	earliest time.Time

	// latest response time form the peer
	latest time.Time

	// number of queries sent to the peer
	nQueries uint64

	// number of queries that resulted an in error
	nErrors uint64
}

func newResponseStats() ResponseRecorder {
	return &responseRecorder{
		earliest: time.Unix(0, 0),
		latest:   time.Unix(0, 0),
		nQueries: 0,
		nErrors:  0,
	}
}

func (rs *responseRecorder) Success() {
	rs.recordResponse()
}

func (rs *responseRecorder) Error() {
	rs.nErrors++
	rs.recordResponse()
}

func (rs *responseRecorder) ToStored() *storage.Responses {
	return &storage.Responses{
		Earliest: rs.earliest.Unix(),
		Latest:   rs.latest.Unix(),
		NQueries: rs.nQueries,
		NErrors:  rs.nErrors,
	}
}

// recordsResponse records any response from the peer.
func (rs *responseRecorder) recordResponse() {
	rs.nQueries++
	rs.latest = time.Now().UTC()
	if rs.earliest.Unix() == 0 {
		rs.earliest = rs.latest
	}
}
