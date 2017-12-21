package peer

import (
	"time"

	"sync"

	"github.com/drausin/libri/libri/librarian/server/storage"
)

// QueryType is a type of query, for now just distinguishing between requests and responses.
type QueryType int

const (
	// Request denotes a request query from the peer.
	Request QueryType = iota

	// Response denotes a query response from the peer.
	Response
)

// Outcome is the outcome of a query, for now just distinguishing between successes and failures.
type Outcome int

const (
	// Success denotes a successful query.
	Success Outcome = iota

	// Error denotes a failed query.
	Error
)

// Recorder tracks statistics associated with queries to/from a peer.
type Recorder interface {

	// Record an outcome for a particular query type.
	Record(t QueryType, o Outcome)

	// merge combines the stats of the other and current recorder instances.
	Merge(other Recorder)

	// ToStored creates a storage.ResponseStats.
	ToStored() *storage.QueryOutcomes
}

type queryRecorder struct {
	requests  *queryTypeOutcomes
	responses *queryTypeOutcomes
}

func newQueryRecorder() *queryRecorder {
	return &queryRecorder{
		requests:  newQueryTypeOutcomes(),
		responses: newQueryTypeOutcomes(),
	}
}

func (qr *queryRecorder) Record(t QueryType, o Outcome) {
	if t == Request {
		qr.requests.Record(o)
		return
	}
	if t == Response {
		qr.responses.Record(o)
		return
	}
}

func (qr *queryRecorder) Merge(other Recorder) {
	qr.requests.Merge(other.(*queryRecorder).requests)
	qr.responses.Merge(other.(*queryRecorder).responses)
}

func (qr *queryRecorder) ToStored() *storage.QueryOutcomes {
	return &storage.QueryOutcomes{
		Requests:  qr.requests.ToStored(),
		Responses: qr.responses.ToStored(),
	}
}

type queryTypeOutcomes struct {
	// earliest response time from the peer
	earliest time.Time

	// latest response time from the peer
	latest time.Time

	// number of queries sent to the peer
	nQueries uint64

	// number of queries that resulted an in error
	nErrors uint64

	mu sync.Mutex
}

func newQueryTypeOutcomes() *queryTypeOutcomes {
	return &queryTypeOutcomes{
		earliest: time.Unix(0, 0).UTC(),
		latest:   time.Unix(0, 0).UTC(),
		nQueries: 0,
		nErrors:  0,
	}
}

func (qto *queryTypeOutcomes) Record(o Outcome) {
	qto.mu.Lock()
	defer qto.mu.Unlock()
	if o == Error {
		qto.nErrors++
	}
	qto.nQueries++
	qto.latest = time.Now().UTC()
	if qto.earliest.Unix() == 0 {
		qto.earliest = qto.latest
	}
}

func (qto *queryTypeOutcomes) Merge(other *queryTypeOutcomes) {
	qto.mu.Lock()
	defer qto.mu.Unlock()
	if qto.earliest.After(other.earliest) {
		qto.earliest = other.earliest
	}
	if qto.latest.Before(other.latest) {
		qto.latest = other.latest
	}
	qto.nQueries += other.nQueries
	qto.nErrors += other.nErrors
}

func (qto *queryTypeOutcomes) ToStored() *storage.QueryTypeOutcomes {
	return &storage.QueryTypeOutcomes{
		Earliest: qto.earliest.Unix(),
		Latest:   qto.latest.Unix(),
		NQueries: qto.nQueries,
		NErrors:  qto.nErrors,
	}
}
