package replicate

import (
	crand "crypto/rand"
	"math/rand"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/drausin/libri/libri/librarian/server/store"
	"github.com/drausin/libri/libri/librarian/server/verify"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	// DefaultVerifyInterval is the default amount of time between verify operations.
	DefaultVerifyInterval = 10 * time.Second

	// DefaultReplicateConcurrency is the default number of replicator routines.
	DefaultReplicateConcurrency = uint(1)

	// DefaultMaxErrRate is the default maximum allowed error rate for verify & store requests
	// before a fatal error is thrown.
	DefaultMaxErrRate = 0.25

	// macKeySize is the size of the MAC key used for verify operations.
	macKeySize = 32

	// errQueueSize is the size of the error queue used to calculate the running error rate.
	errQueueSize = 100

	// underreplicatedQueueSize is the size of the queue of under-replicated documents to be
	// replicated
	underreplicatedQueueSize = 32

	// stopCleanupWaitTime is the amount of time to wait after closing the done channel for
	// the DB iterator to finish
	stopCleanupWaitTime = 1 * time.Second

	// logger keys
	logVerify = "verify"
	logStore  = "store"
)

var (
	metricsKey              = []byte("replicationMetrics")
	errVerifyExhausted      = errors.New("verification failed because exhausted peers to query")
	errMissingStoredMetrics = errors.New("missing stored metrics")
)

// Parameters is the replicator parameters.
type Parameters struct {
	VerifyInterval       time.Duration
	ReplicateConcurrency uint
	MaxErrRate           float32
}

// NewDefaultParameters returns the default replicator parameters.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		VerifyInterval:       DefaultVerifyInterval,
		ReplicateConcurrency: DefaultReplicateConcurrency,
		MaxErrRate:           DefaultMaxErrRate,
	}
}

// Metrics contains (monotonically increasing) counters for different replication events.
type Metrics struct {
	NVerified        uint64
	NUnderreplicated uint64
	NReplicated      uint64
	LatestPass       int64
}

func (m *Metrics) clone() *Metrics {
	return &Metrics{
		NVerified:        m.NVerified,
		NReplicated:      m.NReplicated,
		NUnderreplicated: m.NUnderreplicated,
		LatestPass:       m.LatestPass,
	}
}

// Replicator is a long-running routine that iterates through stored documents and verified that
// they are fully replicated. When they are not, it issues Store requests to close peers to
// bring their replication up to the desired level.
type Replicator interface {
	// Start starts the replicator routines.
	Start() error

	// Stop gracefully stops the replicator routines.
	Stop()

	// Metrics returns a copy of the current metrics.
	Metrics() *Metrics
}

type replicator struct {
	selfID           ecid.ID
	rt               routing.Table
	metrics          *Metrics
	docS             storage.DocumentStorer
	metricsSL        metricsStorerLoader
	verifier         verify.Verifier
	storer           store.Storer
	replicatorParams *Parameters
	verifyParams     *verify.Parameters
	storeParams      *store.Parameters
	underreplicated  chan *verify.Verify
	done             chan struct{}
	errs             chan error
	fatal            chan error
	logger           *zap.Logger
	rng              *rand.Rand
	mu               sync.Mutex
}

// NewReplicator returns a new Replicator.
func NewReplicator(
	selfID ecid.ID,
	rt routing.Table,
	docS storage.DocumentStorer,
	serverSL storage.StorerLoader,
	verifier verify.Verifier,
	storer store.Storer,
	replicatorParams *Parameters,
	verifyParams *verify.Parameters,
	storeParams *store.Parameters,
	rng *rand.Rand,
	logger *zap.Logger,
) Replicator {
	metricsSL := &metricsSLImpl{serverSL}

	// init metrics from stored value, if it exists
	metrics, err := metricsSL.Load()
	if err != nil {
		// if missing or other error, init new metrics
		metrics = &Metrics{}
	}
	return &replicator{
		selfID:           selfID,
		rt:               rt,
		metrics:          metrics,
		docS:             docS,
		metricsSL:        metricsSL,
		verifier:         verifier,
		storer:           storer,
		replicatorParams: replicatorParams,
		verifyParams:     verifyParams,
		storeParams:      storeParams,
		underreplicated:  make(chan *verify.Verify, underreplicatedQueueSize),
		errs:             make(chan error, errQueueSize),
		done:             make(chan struct{}),
		fatal:            make(chan error, 1),
		rng:              rng,
		logger:           logger,
	}
}

func (r *replicator) Start() error {

	// listen for fatal error
	var err error
	go func() {
		err = <-r.fatal
		r.Stop()
	}()

	// monitor non-fatal errors, sending fatal error if too many
	maxErrRate := r.replicatorParams.MaxErrRate
	go cerrors.MonitorRunningErrors(r.errs, r.fatal, errQueueSize, maxErrRate, r.logger)

	// iterates through stored docs in a continuous loop
	go r.verify()

	// replicate under-replicated docs found during replication
	wg := new(sync.WaitGroup)
	for c := uint(0); c < r.replicatorParams.ReplicateConcurrency; c++ {
		wg.Add(1)
		go r.replicate(wg)
	}
	wg.Wait()

	return err
}

func (r *replicator) Stop() {
	select {
	case <-r.done: // already closed
	default:
		close(r.done)
		time.Sleep(stopCleanupWaitTime)
	}
	r.wrapLock(func() {
		close(r.errs)
		close(r.underreplicated)
	})
}

func (r *replicator) Metrics() *Metrics {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.metrics.clone()
}

func (r *replicator) verify() {
	for {
		if r.docS.Metrics().NDocuments == 0 {
			time.Sleep(r.replicatorParams.VerifyInterval)
			continue
		}
		if err := r.docS.Iterate(r.done, r.verifyValue); err != nil {
			r.fatal <- err
		}
		r.wrapLock(func() {
			r.metrics.LatestPass = time.Now().Unix()
			if err := r.metricsSL.Store(r.metrics.clone()); err != nil {
				r.fatal <- err
			}
		})
		select {
		case <-r.fatal:
			return // exit if we've received a fatal error
		case <-r.done:
			return // exit if we're done
		default:
			// otherwise, do another pass
		}
	}
}

func (r *replicator) verifyValue(key id.ID, value []byte) {
	defer time.Sleep(r.replicatorParams.VerifyInterval)
	macKey := make([]byte, macKeySize)
	_, err := crand.Read(macKey)
	cerrors.MaybePanic(err) // should never happen

	v := verify.NewVerify(r.selfID, key, value, macKey, r.verifyParams)
	seeds := r.rt.Peak(key, r.verifyParams.NClosestResponses)
	err = r.verifier.Verify(v, seeds)

	if err != nil { // implies v.Errored()
		r.logger.Debug("document verification errored", zap.Object(logVerify, v))
		r.wrapLock(func() { r.errs <- err })
		return
	}
	if v.Exhausted() {
		r.logger.Debug("verify exhausted peers", zap.Object(logVerify, v))
		r.wrapLock(func() { r.errs <- errVerifyExhausted })
		return
	}

	if v.FullyReplicated() {
		r.logger.Debug("document fully-replicated", zap.Object(logVerify, v))
	} else if v.UnderReplicated() {
		r.wrapLock(func() {
			r.metrics.NUnderreplicated++
			r.underreplicated <- v
		})
		r.logger.Info("document under-replicated", zap.Object(logVerify, v))
	}
	r.wrapLock(func() {
		r.metrics.NVerified++
		r.errs <- nil
	})
	for _, p := range v.Result.Closest.Peers() {
		r.rt.Push(p)
	}
}

func (r *replicator) replicate(wg *sync.WaitGroup) {
	defer wg.Done()
	for v := range r.underreplicated {
		s := newStore(r.selfID, v, *r.storeParams)
		// empty seeds b/c verification has already, in effect, replaced the search component of
		// the store operation
		if err := r.storer.Store(s, []peer.Peer{}); err != nil {
			r.errs <- err
			continue
		}
		if s.Stored() {
			r.wrapLock(func() { r.metrics.NReplicated++ })
			r.logger.Info("stored additional document replicas", zap.Object(logStore, s))
		}
		// for all other non-Stored outcomes for the store, we basically give up and hope to
		// replicate on next pass
		r.wrapLock(func() { r.errs <- nil })
	}
}

func (r *replicator) wrapLock(operation func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	operation()
}

func newStore(selfID ecid.ID, v *verify.Verify, storeParams store.Parameters) *store.Store {
	value := &api.Document{}
	cerrors.MaybePanic(proto.Unmarshal(v.Value, value)) // should never happen
	searchParams := &search.Parameters{
		NClosestResponses: uint(v.Result.Closest.Len()),
	}
	s := store.NewStore(selfID, v.Key, value, searchParams, &storeParams)

	// update number of replicas to store to be just enough to get back to full replication
	storeParams.NReplicas = v.Params.NReplicas - uint(len(v.Result.Replicas))
	s.Search.Params.NMaxErrors = v.Params.NMaxErrors
	s.Search.Params.NClosestResponses = storeParams.NReplicas + s.Search.Params.NMaxErrors

	// construct minimal search result from verify
	s.Search.Params.NMaxErrors = v.Params.NMaxErrors
	s.Search.Result = &search.Result{
		Closest:   v.Result.Closest,
		Unqueried: v.Result.Unqueried,
		Responded: v.Result.Responded,
		Errored:   v.Result.Errored,
	}
	// s.Search.FoundClosestPeers() == true

	return s
}

type metricsStorerLoader interface {
	Store(m *Metrics) error
	Load() (*Metrics, error)
}

type metricsSLImpl struct {
	inner storage.StorerLoader
}

func (sl *metricsSLImpl) Store(m *Metrics) error {
	storageMetrics := &storage.ReplicationMetrics{
		NVerified:        m.NVerified,
		NReplicated:      m.NReplicated,
		NUnderreplicated: m.NUnderreplicated,
		LatestPass:       m.LatestPass,
	}
	metricsBytes, err := proto.Marshal(storageMetrics)
	cerrors.MaybePanic(err) // should never happen
	return sl.inner.Store(metricsKey, metricsBytes)
}

func (sl *metricsSLImpl) Load() (*Metrics, error) {
	storageMetricsBytes, err := sl.inner.Load(metricsKey)
	if err != nil {
		return nil, err
	}
	if storageMetricsBytes == nil {
		return nil, errMissingStoredMetrics
	}
	stored := &storage.ReplicationMetrics{}
	cerrors.MaybePanic(proto.Unmarshal(storageMetricsBytes, stored)) // should never happen
	return &Metrics{
		NVerified:        stored.NVerified,
		NReplicated:      stored.NReplicated,
		NUnderreplicated: stored.NUnderreplicated,
		LatestPass:       stored.LatestPass,
	}, nil
}
