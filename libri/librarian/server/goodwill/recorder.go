package goodwill

import (
	"sync"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	selfPeerLabel  = "self"
	otherPeerLabel = "other"
	endpointLabel  = "endpoint"
	queryTypeLabel = "query_type"
	outcomeLabel   = "outcome"

	counterNamespace = "libri"
	counterSubsystem = "goodwill"
	counterName      = "peer_query_count"
)

// Recorder manages metrics about each peer's endpoint query outcomes.
type Recorder interface {

	// Record the outcome from a given query to/from a peer on the endpoint.
	Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome)

	// Get the query types outcomes on a particular endpoint for a peer.
	Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes
}

// PromRecorder is a Recorder that exposes state via Prometheus metrics.
type PromRecorder interface {
	Recorder

	// Register registers the Prometheus metric(s) with the default Prometheus registerer.
	Register()

	// Unregister unregisters the Prometheus metrics form the default Prometheus registerer.
	Unregister()
}

type scalarRecorder struct {
	peers map[string]EndpointQueryOutcomes
	mu    sync.Mutex
}

// NewScalarRecorder creates a new Recorder that stores scalar metrics about each peer's endpoint
// query outcomes.
func NewScalarRecorder() Recorder {
	return &scalarRecorder{
		peers: make(map[string]EndpointQueryOutcomes),
	}
}

func (r *scalarRecorder) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	if endpoint == api.All {
		panic("cannot record outcome for all endpoints")
	}
	r.mu.Lock()
	if _, in := r.peers[peerID.String()]; !in {
		r.peers[peerID.String()] = newEndpointQueryOutcomes()
	}
	r.mu.Unlock()
	r.record(peerID, endpoint, qt, o)
	r.record(peerID, api.All, qt, o)
}

func (r *scalarRecorder) record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	r.mu.Lock()
	m := r.peers[peerID.String()][endpoint][qt][o]
	r.mu.Unlock()
	m.Record()
}

func (r *scalarRecorder) Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes {
	r.mu.Lock()
	po, in := r.peers[peerID.String()]
	r.mu.Unlock()
	if !in {
		return newQueryOutcomes() // zero values
	}
	return po[endpoint]
}

// NewPromScalarRecorder creates a new scalar recorder that also emits Prometheus metrics for each
// (peer, endpoint, query, outcome).
func NewPromScalarRecorder(selfID id.ID) PromRecorder {
	counter := prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: counterNamespace,
			Subsystem: counterSubsystem,
			Name:      counterName,
			Help:      "Number of queries to and from other peers.",
		},
		[]string{
			selfPeerLabel,
			otherPeerLabel,
			endpointLabel,
			queryTypeLabel,
			outcomeLabel,
		},
	)
	return &promScalarRecorder{
		scalarRecorder: NewScalarRecorder().(*scalarRecorder),
		selfID:         selfID,
		counter:        counter,
	}
}

type promScalarRecorder struct {
	*scalarRecorder

	selfID  id.ID
	counter *prom.CounterVec
}

func (r *promScalarRecorder) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	r.scalarRecorder.Record(peerID, endpoint, qt, o)
	r.counter.With(prom.Labels{
		selfPeerLabel:  id.ShortHex(r.selfID.Bytes()),
		otherPeerLabel: id.ShortHex(peerID.Bytes()),
		endpointLabel:  endpoint.String(),
		queryTypeLabel: qt.String(),
		outcomeLabel:   o.String(),
	}).Inc()
}

func (r *promScalarRecorder) Register() {
	prom.MustRegister(r.counter)
}

func (r *promScalarRecorder) Unregister() {
	_ = prom.Unregister(r.counter)
}
