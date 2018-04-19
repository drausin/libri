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

	counterName = "peer_query_count"
)

type Recorder interface {
	// Record the outcome from a given query to/from a peer on the endpoint.
	Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome)

	// Get the query types outcomes on a particular endpoint for a peer.
	Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes
}

type scalarRecorder struct {
	peers map[string]EndpointQueryOutcomes
	mu    sync.Mutex
}

func NewScalarRecorder() *scalarRecorder {
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

func NewPromScalarRecorder(selfID id.ID) Recorder {
	counter := prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "grpc",
			Subsystem: "server",
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
	prom.MustRegister(counter)
	return &promScalarRecorder{
		scalarRecorder: NewScalarRecorder(),
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
		selfPeerLabel:  r.selfID.String(),
		otherPeerLabel: peerID.String(),
		endpointLabel:  endpoint.String(),
		queryTypeLabel: qt.String(),
		outcomeLabel:   o.String(),
	}).Inc()
}
