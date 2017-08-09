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
	macKeySize = 32

	DefaultVerifyInterval = 250 * time.Millisecond

	DefaultReplicateConcurrency = uint(1)

	// DefaultMaxErrRate is the default maximum allowed error rate for verify & store requests
	// before a fatal error is thrown.
	DefaultMaxErrRate = 0.1

	// errQueueSize is the size of the error queue used to calculate the running error rate.
	errQueueSize = 100
)

var (
	metricsKey         = []byte("replicationMetrics")
	errVerifyExhausted = errors.New("verification failed because exhausted peers to query")
)

type Parameters struct {
	VerifyInterval       time.Duration
	ReplicateConcurrency uint
	MaxErrRate           float32
}

func NewDefaultParameters() *Parameters {
	return &Parameters{
		VerifyInterval:       DefaultVerifyInterval,
		ReplicateConcurrency: DefaultReplicateConcurrency,
		MaxErrRate:           DefaultMaxErrRate,
	}
}

type Metrics struct {
	NUnderreplicated uint64
	NReplicated      uint64
	LatestPass       int64
}

type Replicator interface {
	Start() error
	Stop()
	Metrics() *storage.ReplicationMetrics
}

type replicater struct {
	selfID           ecid.ID
	replicatorParams *Parameters
	verifyParams     *verify.Parameters
	storeParams      *store.Parameters
	verifier         verify.Verifier
	storer           store.Storer
	rt               routing.Table
	docS             storage.DocumentStorer
	metricsSL        storage.StorerLoader
	metrics          *Metrics
	toReplicate      chan *verify.Verify
	done             chan struct{}
	errs             chan error
	fatal            chan error
	logger           *zap.Logger
	rng              *rand.Rand
	mu               sync.Mutex
}

func (r *replicater) Start() error {

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

	select {
	case err := <-r.fatal:
		return err
	case <-r.done:
		return nil
	}
}

func (r *replicater) Stop() {
	close(r.toReplicate)
	select {
	case <-r.errs: // already closed
	default:
		close(r.errs)
	}
	select {
	case <-r.done: // already closed
	default:
		close(r.done)
	}
}

func (r *replicater) Metrics() *Metrics {
	return r.metrics
}

func (r *replicater) verify() {
	for {
		if err := r.docS.Iterate(r.done, r.verifyValue); err != nil {
			r.fatal <- err
		}
		r.wrapLock(func() {
			r.metrics.LatestPass = time.Now().Unix()
			if err := r.saveMetrics(); err != nil {
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

func (r *replicater) verifyValue(key id.ID, value []byte) {
	macKey := make([]byte, macKeySize)
	_, err := crand.Read(macKey)
	cerrors.MaybePanic(err) // should never happen

	v := verify.NewVerify(r.selfID, key, value, macKey, r.verifyParams)
	seeds := r.rt.Peak(key, r.verifyParams.Concurrency)
	err = r.verifier.Verify(v, seeds)

	if err != nil {
		r.errs <- err
	} else if v.Exhausted() {
		r.errs <- errVerifyExhausted
	} else if v.UnderReplicated() {
		// if we're under-replicated, add to replication queue
		r.toReplicate <- v
		r.wrapLock(func() { r.metrics.NUnderreplicated++ })
	}

	time.Sleep(r.replicatorParams.VerifyInterval)
}

func (r *replicater) replicate(wg *sync.WaitGroup) {
	defer wg.Done()
	for v := range r.toReplicate {
		s := newStore(r.selfID, v, v.Value, *r.storeParams)
		// empty seeds b/c verification has already, in effect, replaced the search component of
		// the store operation
		r.errs <- r.storer.Store(s, []peer.Peer{})
		if s.Stored() {
			r.wrapLock(func() { r.metrics.NReplicated++ })
		}
		// for all other non-Stored outcomes for the store, we basically give up and hope to
		// replicate on next pass
	}
}

func (r *replicater) saveMetrics() error {
	storageMetrics := &storage.ReplicationMetrics{
		NReplicated:      r.metrics.NReplicated,
		NUnderreplicated: r.metrics.NUnderreplicated,
		LatestPass:       r.metrics.LatestPass,
	}
	metricsBytes, err := proto.Marshal(storageMetrics)
	if err != nil {
		return err
	}
	return r.metricsSL.Store(metricsKey, metricsBytes)
}

func (r *replicater) wrapLock(operation func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	operation()
}

func newStore(selfID ecid.ID, v *verify.Verify, valueBytes []byte, storeParams store.Parameters) *store.Store {
	var value *api.Document
	cerrors.MaybePanic(proto.Unmarshal(valueBytes, value)) // should never happen
	searchParams := &search.Parameters{
		NClosestResponses: uint(v.Result.Closest.Len()),
	}
	s := store.NewStore(selfID, v.Key, value, searchParams, &storeParams)

	// update number of replicas to store to be just enough to get back to full replication
	storeParams.NReplicas = v.Params.NReplicas - uint(len(v.Result.Replicas))

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
