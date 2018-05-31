package comm

import (
	"sync"

	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	prom "github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

var (
	// healthyErrStatusCodes defines the set of GRPC error codes that a health server can
	// return. usually due to some client issue.
	healthyErrStatusCodes = map[codes.Code]struct{}{
		codes.InvalidArgument:   {},
		codes.PermissionDenied:  {},
		codes.ResourceExhausted: {},
	}
)

// QueryRecorder records metrics about each peer's endpoint query outcomes.
type QueryRecorder interface {

	// Record the outcome from a given query to/from a peer on the endpoint.
	Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome)
}

// MaybeRecordRpErr records skips recording an error using th given QueryRecorder if the given error
// has a health error status code (indicating that the problem is on the client end).
func MaybeRecordRpErr(r QueryRecorder, peerID id.ID, endpoint api.Endpoint, err error) {
	if peerID == nil {
		return
	}
	if errSt, ok := status.FromError(err); ok {
		if _, in := healthyErrStatusCodes[errSt.Code()]; in {
			r.Record(peerID, endpoint, Response, Success)
			return
		}
	}
	r.Record(peerID, endpoint, Response, Error)
}

// QueryGetter gets metrics about each peer's endpoint query outcomes.
type QueryGetter interface {

	// Get the query types outcomes on a particular endpoint for a peer.
	Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes

	// CountPeers returns the number of peers with queries on the given endpoint on the given
	// type.
	CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int
}

// QueryRecorderGetter both records and gets metrics about each peer's endpoint query outcomes.
type QueryRecorderGetter interface {
	QueryRecorder
	QueryGetter
}

type scalarRG struct {
	peers              map[string]endpointQueryOutcomes
	endpointQueryPeers endpointQueryPeers
	knower             Knower
	mu                 sync.Mutex
}

// NewQueryRecorderGetter creates a new QueryRecorder that stores scalar metrics about each peer's
// endpoint query outcomes.
func NewQueryRecorderGetter(knower Knower) QueryRecorderGetter {
	return &scalarRG{
		peers:              make(map[string]endpointQueryOutcomes),
		endpointQueryPeers: newEndpointQueryPeers(),
		knower:             knower,
	}
}

func (r *scalarRG) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
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

func (r *scalarRG) record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	idStr := peerID.String()
	known := r.knower.Know(peerID)
	r.mu.Lock()
	m := r.peers[idStr][endpoint][qt][o]
	r.endpointQueryPeers[endpoint][qt][known][idStr] = struct{}{}
	r.mu.Unlock()
	m.Record()
}

func (r *scalarRG) Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes {
	r.mu.Lock()
	po, in := r.peers[peerID.String()]
	r.mu.Unlock()
	if !in {
		return newQueryOutcomes() // zero values
	}
	return po[endpoint]
}

func (r *scalarRG) CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.endpointQueryPeers[endpoint][qt][known])
}

// WindowQueryRecorders contains a collection of QueryRecorders, each defined over a specific time
// window.
type WindowQueryRecorders map[time.Duration]QueryRecorder

// WindowQueryGetters contains a collection of QueryGetters, each defined over a specific time
// window.
type WindowQueryGetters map[time.Duration]QueryGetter

// Record the outcome from a given query to/from a peer on the endpoint.
func (rs WindowQueryRecorders) Record(
	peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome,
) {
	for _, r := range rs {
		r.Record(peerID, endpoint, qt, o)
	}
}

// NewWindowQueryRecorderGetters returns a single QueryRecorder wrapper a set of individual
// QueryRecorderGetters, one for each time window. It also returns WindowQueryGetters containing
// QueryGetters for each time window.
func NewWindowQueryRecorderGetters(
	knower Knower, windows []time.Duration,
) (QueryRecorder, WindowQueryGetters) {
	recorders := WindowQueryRecorders{}
	getters := WindowQueryGetters{}
	for _, window := range windows {
		rg := NewWindowRecorderGetter(knower, window)
		recorders[window] = rg
		getters[window] = rg
	}
	return recorders, getters
}

// NewWindowRecorderGetter returns a WindowRecorder using the given Knower and window size.
func NewWindowRecorderGetter(knower Knower, window time.Duration) QueryRecorderGetter {
	return &windowRG{
		scalarRG: NewQueryRecorderGetter(knower).(*scalarRG),
		window:   window,
	}
}

type windowRG struct {
	*scalarRG

	window time.Duration
	start  time.Time
	end    time.Time
}

func (r *windowRG) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	r.maybeNextWindow()
	r.scalarRG.Record(peerID, endpoint, qt, o)
}

func (r *windowRG) Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes {
	r.maybeNextWindow()
	return r.scalarRG.Get(peerID, endpoint)
}

func (r *windowRG) CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int {
	r.maybeNextWindow()
	return r.scalarRG.CountPeers(endpoint, qt, known)
}

func (r *windowRG) maybeNextWindow() {
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

// PromRecorder is a QueryRecorder that exposes state via Prometheus metrics.
type PromRecorder interface {
	QueryRecorder

	// Register registers the Prometheus metric(s) with the default Prometheus registerer.
	Register()

	// Unregister unregisters the Prometheus metrics form the default Prometheus registerer.
	Unregister()
}

// NewPromScalarRecorder creates a new scalar recorder that also emits Prometheus metrics for each
// (peer, endpoint, query, outcome).
func NewPromScalarRecorder(selfID id.ID, inner QueryRecorder) PromRecorder {
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
	return &promQR{
		inner:   inner,
		selfID:  selfID,
		counter: counter,
	}
}

type promQR struct {
	inner QueryRecorder

	selfID  id.ID
	counter *prom.CounterVec
}

func (r *promQR) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {
	r.inner.Record(peerID, endpoint, qt, o)
	r.counter.With(prom.Labels{
		selfPeerLabel:  id.ShortHex(r.selfID.Bytes()),
		otherPeerLabel: id.ShortHex(peerID.Bytes()),
		endpointLabel:  endpoint.String(),
		queryTypeLabel: qt.String(),
		outcomeLabel:   o.String(),
	}).Inc()
}

func (r *promQR) Register() {
	prom.MustRegister(r.counter)
}

func (r *promQR) Unregister() {
	_ = prom.Unregister(r.counter)
}
