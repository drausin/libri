package goodwill

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

type Metrics struct {
	// Earliest response time from the peer
	Earliest time.Time

	// Latest response time from the peer
	Latest time.Time

	// Count of queries sent to the peer
	Count uint64

	mu sync.Mutex
}

func (m *Metrics) Record() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Count++
	m.Latest = time.Now()
	if m.Earliest.IsZero() {
		m.Earliest = m.Latest
	}
}

type QueryOutcomes map[QueryType]map[Outcome]*Metrics

func newQueryOutcomes() QueryOutcomes {
	return QueryOutcomes{
		Request: map[Outcome]*Metrics{
			Success: {},
			Error:   {},
		},
		Response: map[Outcome]*Metrics{
			Success: {},
			Error:   {},
		},
	}
}

type EndpointQueryOutcomes map[api.Endpoint]QueryOutcomes

func newEndpointQueryOutcomes() EndpointQueryOutcomes {
	eqos := EndpointQueryOutcomes{
		api.All: newQueryOutcomes(),
	}
	for _, e := range api.Endpoints {
		eqos[e] = newQueryOutcomes()
	}
	return eqos
}
