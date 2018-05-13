package comm

import (
	"sync"

	"time"

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

	// CountPeers returns the number of peers with queries on the given endpoint on the given
	// type.
	CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int
}

type scalarRecorder struct {
	peers              map[string]endpointQueryOutcomes
	endpointQueryPeers endpointQueryPeers
	knower             Knower
	mu                 sync.Mutex
}

// NewScalarRecorder creates a new Recorder that stores scalar metrics about each peer's endpoint
// query outcomes.
func NewScalarRecorder(knower Knower) Recorder {
	return &scalarRecorder{
		peers:              make(map[string]endpointQueryOutcomes),
		endpointQueryPeers: newEndpointQueryPeers(),
		knower:             knower,
	}
}

func (r *scalarRecorder) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	if endpoint == api.All {
		panic("cannot record outcome for all endpoints")
	}
	r.mu.Lock()
	idStr := peerID.String()
	if _, in := r.peers[idStr]; !in {
		r.peers[idStr] = newEndpointQueryOutcomes()
	}
	r.mu.Unlock()
	r.record(peerID, endpoint, qt, o)
	r.record(peerID, api.All, qt, o)
}

func (r *scalarRecorder) record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	idStr := peerID.String()
	known := r.knower.Know(peerID)
	r.mu.Lock()
	m := r.peers[idStr][endpoint][qt][o]
	r.endpointQueryPeers[endpoint][qt][known][idStr] = struct{}{}
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

func (r *scalarRecorder) CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.endpointQueryPeers[endpoint][qt][known])
}

// WindowRecorder is a Recorder whose counts reset after a configurable window (e.g., 1 second,
// 1 day).
type WindowRecorder interface {
	Recorder
}

// NewWindowScalarRecorder returns a WindowRecorder using the given Knower and window size.
func NewWindowScalarRecorder(knower Knower, window time.Duration) WindowRecorder {
	return &windowScalarRec{
		scalarRecorder: NewScalarRecorder(knower).(*scalarRecorder),
		window:         window,
	}
}

type windowScalarRec struct {
	*scalarRecorder

	window time.Duration
	start  time.Time
	end    time.Time
}

func (r *windowScalarRec) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	r.maybeNextWindow()
	r.scalarRecorder.Record(peerID, endpoint, qt, o)
}

func (r *windowScalarRec) Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes {
	r.maybeNextWindow()
	return r.scalarRecorder.Get(peerID, endpoint)
}

func (r *windowScalarRec) CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int {
	r.maybeNextWindow()
	return r.scalarRecorder.CountPeers(endpoint, qt, known)
}

func (r *windowScalarRec) maybeNextWindow() {
	r.mu.Lock()
	now := time.Now()
	if now.After(r.end) {

		// reset window
		r.start = now.Round(r.window)
		if r.start.After(now) {
			// start always in past
			r.start = r.start.Add(-r.window)
		}
		r.end = r.start.Add(r.window)

		// reset internal count state
		r.peers = make(map[string]endpointQueryOutcomes)
		r.endpointQueryPeers = newEndpointQueryPeers()
	}
	r.mu.Unlock()
}

// PromRecorder is a Recorder that exposes state via Prometheus metrics.
type PromRecorder interface {
	Recorder

	// Register registers the Prometheus metric(s) with the default Prometheus registerer.
	Register()

	// Unregister unregisters the Prometheus metrics form the default Prometheus registerer.
	Unregister()
}

// NewPromScalarRecorder creates a new scalar recorder that also emits Prometheus metrics for each
// (peer, endpoint, query, outcome).
func NewPromScalarRecorder(selfID id.ID, knower Knower) PromRecorder {
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
		scalarRecorder: NewScalarRecorder(knower).(*scalarRecorder),
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
