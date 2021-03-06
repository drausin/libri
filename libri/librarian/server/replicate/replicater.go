package replicate

import (
	crand "crypto/rand"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/drausin/libri/libri/common/ecid"
	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
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
	DefaultVerifyInterval = 1 * time.Second

	// DefaultReplicateConcurrency is the default number of replicator routines.
	DefaultReplicateConcurrency = uint(1)

	// DefaultVerifyTimeout is the default timeout for the replicator's verify retries.
	DefaultVerifyTimeout = 60 * time.Second

	// DefaultMaxErrRate is the default maximum allowed error rate for verify & store requests
	// before a fatal error is thrown.
	DefaultMaxErrRate = 0.5

	// DefaultReportMetrics is the default setting for whether the replicator reports Prometheus
	// metrics.
	DefaultReportMetrics = true

	// macKeySize is the size of the MAC key used for verify operations.
	macKeySize = 32

	// errQueueSize is the size of the error queue used to calculate the running error rate.
	errQueueSize = 100

	// underreplicatedQueueSize is the size of the queue of under-replicated documents to be
	// replicated
	underreplicatedQueueSize = 32

	// logger keys
	logVerify = "verify"
	logStore  = "store"
)

var (
	errVerifyExhausted = errors.New("verification failed because exhausted peers to query")
)

// Parameters is the replicator parameters.
type Parameters struct {
	VerifyInterval       time.Duration
	ReplicateConcurrency uint
	VerifyTimeout        time.Duration
	MaxErrRate           float32
	ReportMetrics        bool
}

// NewDefaultParameters returns the default replicator parameters.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		VerifyInterval:       DefaultVerifyInterval,
		ReplicateConcurrency: DefaultReplicateConcurrency,
		VerifyTimeout:        DefaultVerifyTimeout,
		MaxErrRate:           DefaultMaxErrRate,
		ReportMetrics:        DefaultReportMetrics,
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
}

type replicator struct {
	peerID           ecid.ID
	orgID            ecid.ID
	rt               routing.Table
	docS             storage.DocumentStorer
	verifier         verify.Verifier
	storer           store.Storer
	replicatorParams *Parameters
	verifyParams     *verify.Parameters
	storeParams      *store.Parameters
	metrics          *metrics
	underreplicated  chan *verify.Verify
	stop             chan struct{}
	stopped          chan struct{}
	errs             chan error
	fatal            chan error
	logger           *zap.Logger
	rng              *rand.Rand
	mu               sync.Mutex
}

// NewReplicator returns a new Replicator.
func NewReplicator(
	peerID ecid.ID,
	orgID ecid.ID,
	rt routing.Table,
	docS storage.DocumentStorer,
	verifier verify.Verifier,
	storer store.Storer,
	replicatorParams *Parameters,
	verifyParams *verify.Parameters,
	storeParams *store.Parameters,
	rng *rand.Rand,
	logger *zap.Logger,
) Replicator {
	return &replicator{
		peerID:           peerID,
		orgID:            orgID,
		rt:               rt,
		docS:             docS,
		verifier:         verifier,
		storer:           storer,
		replicatorParams: replicatorParams,
		verifyParams:     verifyParams,
		storeParams:      storeParams,
		metrics:          newMetrics(),
		underreplicated:  make(chan *verify.Verify, underreplicatedQueueSize),
		errs:             make(chan error, errQueueSize),
		stop:             make(chan struct{}),
		stopped:          make(chan struct{}),
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

	if r.replicatorParams.ReportMetrics {
		r.metrics.register()
	}

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
	r.logger.Info("ending replicator")
	safeClose(r.stop)
	r.wrapLock(func() {
		safeCloseErrChan(r.errs)
		safeCloseVerifyChan(r.underreplicated)
	})
	if r.replicatorParams.ReportMetrics {
		r.metrics.unregister()
	}
	<-r.stopped
	r.logger.Debug("ended replicator")
}

func (r *replicator) verify() {
	rng := rand.New(rand.NewSource(int64(r.rng.Int())))
	for {
		// pause for documents to be added/things to change a bit before next verify iteration
		pause := make(chan struct{})
		go func() {
			intervalMaxMs := int(r.replicatorParams.VerifyInterval / time.Millisecond)
			waitMs := time.Duration(rng.Intn(intervalMaxMs))
			time.Sleep(waitMs * time.Millisecond)
			close(pause)
		}()
		select {
		case <-r.stop:
			close(r.stopped)
			return
		case <-pause:
		}

		if err := r.docS.Iterate(r.stop, r.verifyValue); err != nil {
			r.fatal <- err
		}

		select {
		case <-r.stop: // exit if we've received stop signal
			close(r.stopped)
			return
		default:
			// otherwise, do another pass
		}
	}
}

func (r *replicator) verifyValue(key id.ID, value []byte) {
	pause := make(chan struct{})
	go func() {
		time.Sleep(r.replicatorParams.VerifyInterval)
		close(pause)
	}()
	macKey := make([]byte, macKeySize)
	_, err := crand.Read(macKey)
	cerrors.MaybePanic(err) // should never happen

	v := verify.NewVerify(r.peerID, r.orgID, key, value, macKey, r.verifyParams)
	seeds := r.rt.Find(key, r.verifyParams.NClosestResponses)

	operation := func() error {
		v.Result = verify.NewInitialResult(key, r.verifyParams)
		return r.verifier.Verify(v, seeds)
	}
	err = backoff.Retry(operation, client.NewExpBackoff(r.replicatorParams.VerifyTimeout))

	if err != nil { // implies v.Errored()
		r.logger.Error("document verification errored", zap.Object(logVerify, v))
		r.metrics.incVerification(errored, unknown)
		r.wrapLock(func() { maybeSendErrChan(r.errs, err) })
		return
	}
	if v.Exhausted() {
		r.logger.Error("verify exhausted peers", zap.Object(logVerify, v))
		r.metrics.incVerification(exhausted, unknown)
		r.wrapLock(func() { maybeSendErrChan(r.errs, errVerifyExhausted) })
		return
	}

	if v.FullyReplicated() {
		r.logger.Debug("document fully-replicated", zap.Object(logVerify, v))
		r.metrics.incVerification(succeeded, full)
	} else if v.UnderReplicated() {
		r.logger.Info("document under-replicated", zap.Object(logVerify, v))
		r.metrics.incVerification(succeeded, under)
		r.wrapLock(func() { maybeSendVerifyChan(r.underreplicated, v) })
	}
	r.wrapLock(func() { maybeSendErrChan(r.errs, nil) })
	for _, p := range v.Result.Responded {
		r.rt.Push(p)
	}
	select {
	case <-r.stop:
	case <-pause:
	}
}

func (r *replicator) replicate(wg *sync.WaitGroup) {
	defer wg.Done()
	for v := range r.underreplicated {
		s := newStore(r.peerID, r.orgID, v, *r.storeParams)
		// empty seeds b/c verification has already, in effect, replaced the search component of
		// the store operation
		if err := r.storer.Store(s, []peer.Peer{}); err != nil {
			r.metrics.incReplication(errored)
			r.logger.Error("replication store failed", zap.Object(logStore, s))
			maybeSendErrChan(r.errs, err)
			continue
		}
		if s.Stored() {
			r.metrics.incReplication(succeeded)
			r.logger.Info("stored additional replicas", zap.Object(logStore, s))
		} else {
			r.metrics.incReplication(errored)
			r.logger.Error("failed to store additional replicas", zap.Object(logStore, s))
		}
		// for all other non-Stored outcomes for the store, we basically give up and hope to
		// replicate on next pass
		r.wrapLock(func() { maybeSendErrChan(r.errs, nil) })
	}
}

func (r *replicator) wrapLock(operation func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	operation()
}

func newStore(peerID, orgID ecid.ID, v *verify.Verify, storeParams store.Parameters) *store.Store {
	value := &api.Document{}
	cerrors.MaybePanic(proto.Unmarshal(v.Value, value)) // should never happen
	searchParams := &search.Parameters{
		NClosestResponses: uint(v.Result.Closest.Len()),
	}
	s := store.NewStore(peerID, orgID, v.Key, value, searchParams, &storeParams)

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

func safeClose(ch chan struct{}) {
	select {
	case <-ch: // already closed
	default:
		close(ch)
	}
}

func safeCloseErrChan(ch chan error) {
	select {
	case <-ch: // already closed
	default:
		close(ch)
	}
}

func safeCloseVerifyChan(ch chan *verify.Verify) {
	select {
	case <-ch: // already closed
	default:
		close(ch)
	}
}

func maybeSendErrChan(ch chan error, err error) {
	select {
	case prev, open := <-ch:
		if open {
			ch <- prev
			ch <- err
		}
	default:
		ch <- err
	}
}

func maybeSendVerifyChan(ch chan *verify.Verify, v *verify.Verify) {
	select {
	case prev, open := <-ch:
		if open {
			ch <- prev
			ch <- v
		}
	default:
		ch <- v
	}
}
