package comm

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
)

// QueryType is a type of query, for now just distinguishing between requests and responses.
type QueryType int

const (
	// Request denotes a request query from the peer.
	Request QueryType = iota

	// Response denotes a query response from the peer.
	Response
)

// String returns a string representation of the query type.
func (qt QueryType) String() string {
	switch qt {
	case Request:
		return "REQUEST"
	case Response:
		return "RESPONSE"
	default:
		panic("unknown query type")
	}
}

// Outcome is the outcome of a query, for now just distinguishing between successes and failures.
type Outcome int

const (
	// Success denotes a successful query.
	Success Outcome = iota

	// Error denotes a failed query.
	Error
)

// String returns a string representation of the outcome.
func (o Outcome) String() string {
	switch o {
	case Success:
		return "SUCCESS"
	case Error:
		return "ERROR"
	default:
		panic("unknown outcome")
	}
}

// ScalarMetrics contains scalar metrics for a given query type and outcome.
type ScalarMetrics struct {

	// Earliest response time from the peer
	Earliest time.Time

	// Latest response time from the peer
	Latest time.Time

	// CountPeers of queries sent to the peer
	Count uint64

	mu *sync.Mutex
}

func newScalarMetrics() *ScalarMetrics {
	return &ScalarMetrics{
		mu: new(sync.Mutex),
	}
}

// Record updates the metrics to mark a query.
func (m *ScalarMetrics) Record() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Count++
	m.Latest = time.Now()
	if m.Earliest.IsZero() {
		m.Earliest = m.Latest
	}
}

// QueryOutcomes contains the metrics for the 4 (query type, outcome) tuples.
type QueryOutcomes map[QueryType]map[Outcome]*ScalarMetrics

func newQueryOutcomes() QueryOutcomes {
	return QueryOutcomes{
		Request: map[Outcome]*ScalarMetrics{
			Success: newScalarMetrics(),
			Error:   newScalarMetrics(),
		},
		Response: map[Outcome]*ScalarMetrics{
			Success: newScalarMetrics(),
			Error:   newScalarMetrics(),
		},
	}
}

// endpointQueryOutcomes contains the query outcomes for each endpoint.
type endpointQueryOutcomes map[api.Endpoint]QueryOutcomes

func newEndpointQueryOutcomes() endpointQueryOutcomes {
	eqos := endpointQueryOutcomes{
		api.All: newQueryOutcomes(),
	}
	for _, e := range api.Endpoints {
		eqos[e] = newQueryOutcomes()
	}
	return eqos
}

type knownPeers map[bool]map[string]struct{}

type endpointQueryPeers map[api.Endpoint]map[QueryType]knownPeers

func newEndpointQueryPeers() endpointQueryPeers {
	eqp := endpointQueryPeers{
		api.All: newQueryPeers(),
	}
	for _, e := range api.Endpoints {
		eqp[e] = newQueryPeers()
	}
	return eqp
}

func newQueryPeers() map[QueryType]knownPeers {
	return map[QueryType]knownPeers{
		Request: {
			true:  {},
			false: {},
		},
		Response: {
			true:  {},
			false: {},
		},
	}
}
