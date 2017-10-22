package replicate

import (
	"container/heap"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/ecid"
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
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewDefaultParameters(t *testing.T) {
	p := NewDefaultParameters()
	assert.NotZero(t, p.VerifyInterval)
	assert.NotZero(t, p.ReplicateConcurrency)
	assert.NotZero(t, p.MaxErrRate)
}

func TestVerifyDefaultParameters(t *testing.T) {
	p1 := verify.NewDefaultParameters()
	p2 := NewVerifyDefaultParameters()
	assert.True(t, p1.NReplicas == p2.NReplicas+1)
	assert.True(t, p1.NClosestResponses == p2.NClosestResponses+1)
	assert.True(t, p2.ExcludeSelf)
}

func TestMetrics_clone(t *testing.T) {
	m1 := &Metrics{
		NVerified:        2,
		NUnderreplicated: 2,
		NReplicated:      2,
		LatestPass:       time.Now().Unix(),
	}
	m2 := m1.clone()
	assert.Equal(t, m1, m2)
}

func TestReplicator_StartStop(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	kvdb, cleanup, err := db.NewTempDirRocksDB()
	assert.Nil(t, err)
	defer cleanup()
	defer kvdb.Close()

	rt, selfID, _ := routing.NewTestWithPeers(rng, 10)
	docS := storage.NewDocumentSLD(kvdb)
	serverSL := storage.NewServerSL(kvdb)
	verifyParams := verify.NewDefaultParameters()
	replicatorParams := &Parameters{
		VerifyInterval:       10 * time.Millisecond,
		ReplicateConcurrency: 3,
		MaxErrRate:           DefaultMaxErrRate,
	}
	storeParams := store.NewDefaultParameters()

	// add some docs
	nDocs := 3
	for c := 0; c < nDocs; c++ {
		value, key := api.NewTestDocument(rng)
		err = docS.Store(key, value)
		assert.Nil(t, err)
	}

	// create some dummy unqueried and closest heaps for our fixed result to use
	key := id.NewPseudoRandom(rng)
	unqueried := search.NewClosestPeers(key, 10)
	err = unqueried.SafePushMany(peer.NewTestPeers(rng, 10))
	assert.Nil(t, err)
	closest := search.NewFarthestPeers(key, 6)
	for c := uint(0); c < verifyParams.NClosestResponses; c++ {
		heap.Push(closest, heap.Pop(unqueried).(peer.Peer))
	}
	replicas := make(map[string]peer.Peer)
	for _, p := range peer.NewTestPeers(rng, int(verifyParams.NReplicas-1)) {
		replicas[p.ID().String()] = p
	}

	// verifier result is always under-replicated
	verifier := &fixedVerifier{
		result: &verify.Result{
			Replicas:  replicas,
			Unqueried: unqueried,
			Closest:   closest,
		},
	}

	storer := &fixedStorer{
		result: &store.Result{Responded: peer.NewTestPeers(rng, int(verifyParams.NReplicas))},
	}

	r := NewReplicator(
		selfID,
		rt,
		docS,
		serverSL,
		verifier,
		storer,
		replicatorParams,
		verifyParams,
		storeParams,
		rng,
		zap.NewNop(),
	)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		err = r.Start()
		assert.Nil(t, err)
	}(wg)

	time.Sleep(100 * time.Millisecond)
	r.Stop()

	m := r.Metrics()
	assert.True(t, m.NVerified > 0)
	assert.True(t, m.NReplicated > 0)
	assert.True(t, m.NUnderreplicated > 0)
	assert.True(t, m.NVerified >= m.NReplicated)
	assert.True(t, m.LatestPass > 0)

	wg.Wait()

	// now make the storer only error
	storer = &fixedStorer{
		err: errors.New("some Store error"),
	}
	r = NewReplicator(
		selfID,
		rt,
		docS,
		serverSL,
		verifier,
		storer,
		replicatorParams,
		verifyParams,
		storeParams,
		rng,
		zap.NewNop(),
	)

	// check that replicator ends on its own with fatal error
	err = r.Start()
	assert.NotNil(t, err)

	// give time for other routines to finish
	time.Sleep(1 * time.Second)
}

func TestReplicator_verify(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	rt, selfID, _ := routing.NewTestWithPeers(rng, 10)
	innerMetricsSL := &storage.TestSLD{}
	r := replicator{
		selfID:           selfID,
		verifyParams:     verify.NewDefaultParameters(),
		replicatorParams: &Parameters{VerifyInterval: 10 * time.Millisecond},
		storeParams:      store.NewDefaultParameters(),
		metrics:          &Metrics{},
		docS:             storage.NewTestDocSLD(),
		metricsSL:        &metricsSLImpl{inner: innerMetricsSL},
		underreplicated:  make(chan *verify.Verify, 1),
		errs:             make(chan error, 8),
		stop:             make(chan struct{}),
		stopped:          make(chan struct{}),
		fatal:            make(chan error, 1),
		rt:               rt,
		logger:           zap.NewNop(), // server.NewDevLogger(zap.DebugLevel),
	}

	// add some docs
	nDocs := 3
	for c := 0; c < nDocs; c++ {
		value, key := api.NewTestDocument(rng)
		err := r.docS.Store(key, value)
		assert.Nil(t, err)
	}
	assert.Equal(t, uint64(nDocs), r.docS.Metrics().NDocuments)

	// verifier always returns results where FullyReplicated() == true
	key := id.NewPseudoRandom(rng) // arbitrary, but just need something
	unqueried := search.NewClosestPeers(key, 10)
	err := unqueried.SafePushMany(peer.NewTestPeers(rng, 10))
	assert.Nil(t, err)
	replicas := make(map[string]peer.Peer)
	for _, p := range peer.NewTestPeers(rng, int(r.verifyParams.NReplicas)) {
		replicas[p.ID().String()] = p
	}
	r.verifier = &fixedVerifier{
		result: &verify.Result{
			Unqueried: unqueried,
			Replicas:  replicas,
			Closest:   search.NewFarthestPeers(key, r.verifyParams.NClosestResponses),
		},
	}

	// start error handling loop
	go func() {
		for err := range r.errs {
			assert.Nil(t, err)
		}
	}()

	// start verify loop
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		r.verify()
	}(wg)

	passComplete := false
	for c := 0; c < 10 && !passComplete; c++ {
		r.wrapLock(func() {
			if r.metrics.LatestPass > 0 {
				passComplete = true
			}
		})
		time.Sleep(100 * time.Millisecond)
	}
	assert.True(t, passComplete)
	m1, err := r.metricsSL.Load()
	assert.Nil(t, err)
	assert.True(t, m1.NVerified > 0)
	assert.Zero(t, m1.NUnderreplicated)
	assert.Zero(t, m1.NReplicated)
	assert.True(t, m1.LatestPass > 0)

	m2 := r.Metrics()
	assert.Equal(t, m1.LatestPass, m2.LatestPass)
	assert.True(t, m2.NVerified >= m1.NVerified)
	assert.Zero(t, m2.NUnderreplicated)
	assert.Zero(t, m2.NReplicated)

	close(r.stop)

	// wait for verify loop to finish
	wg.Wait()
}

func TestReplicator_verifyValue(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value, key := api.NewTestDocument(rng)
	valueBytes, err := proto.Marshal(value)
	assert.Nil(t, err)
	rt, selfID, _ := routing.NewTestWithPeers(rng, 10)
	replicatorParams := &Parameters{
		VerifyInterval: 10 * time.Millisecond,
		VerifyTimeout:  10 * time.Millisecond,
	}
	r := replicator{
		selfID:           selfID,
		verifyParams:     verify.NewDefaultParameters(),
		replicatorParams: replicatorParams,
		storeParams:      store.NewDefaultParameters(),
		metrics:          &Metrics{},
		underreplicated:  make(chan *verify.Verify, 1),
		errs:             make(chan error, 1),
		rt:               rt,
		logger:           zap.NewNop(),
	}
	unqueried := search.NewClosestPeers(key, 10)
	err = unqueried.SafePushMany(peer.NewTestPeers(rng, 10))
	assert.Nil(t, err)

	// check that when a verify operation has FullyReplicated() == true, only a nil error is
	// sent to errs
	r.verifier = &fixedVerifier{
		result: &verify.Result{
			Replicas:  peerMap(peer.NewTestPeers(rng, int(r.verifyParams.NReplicas))),
			Unqueried: unqueried,
			Closest:   search.NewFarthestPeers(key, r.verifyParams.NClosestResponses),
		},
	}
	r.verifyValue(key, valueBytes)
	err = <-r.errs
	assert.Nil(t, err)
	r.wrapLock(func() { assert.Zero(t, r.metrics.NUnderreplicated) })
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NVerified) })
	select {
	case <-r.underreplicated:
		assert.True(t, false) // should't get message in underreplicated
	default:
	}

	// check that when a verify operation has UnderReplicated() == true, get msg in toReplicated
	// and nil error
	closest := search.NewFarthestPeers(key, 6)
	for c := uint(0); c < r.verifyParams.NClosestResponses; c++ {
		heap.Push(closest, heap.Pop(unqueried).(peer.Peer))
	}
	r.verifier = &fixedVerifier{
		result: &verify.Result{
			Replicas:  peerMap(peer.NewTestPeers(rng, int(r.verifyParams.NReplicas-1))),
			Unqueried: unqueried,
			Closest:   closest,
		},
	}
	r.verifyValue(key, valueBytes)
	err = <-r.errs
	assert.Nil(t, err)
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NUnderreplicated) })
	r.wrapLock(func() { assert.Equal(t, uint64(2), r.metrics.NVerified) })
	v := <-r.underreplicated
	assert.Equal(t, key, v.Key)

	// check that when verify is exhausted, we get an error
	for unqueried.Len() > 0 {
		heap.Pop(unqueried)
	}
	r.verifier = &fixedVerifier{
		result: &verify.Result{
			Replicas:  make(map[string]peer.Peer),
			Unqueried: unqueried,
			Closest:   search.NewFarthestPeers(key, r.verifyParams.NClosestResponses),
		},
	}
	r.verifyValue(key, valueBytes)
	err = <-r.errs
	assert.Equal(t, errVerifyExhausted, err)
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NUnderreplicated) })
	r.wrapLock(func() { assert.Equal(t, uint64(2), r.metrics.NVerified) })
	select {
	case <-r.underreplicated:
		assert.True(t, false) // should't get message in underreplicated
	default:
	}

	// check that when verify errors, we get an error
	r.verifier = &fixedVerifier{
		err:    errors.New("some Verify error"),
		result: verify.NewInitialResult(key, verify.NewDefaultParameters()),
	}
	r.verifyValue(key, valueBytes)
	err = <-r.errs
	assert.NotNil(t, err)
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NUnderreplicated) })
	r.wrapLock(func() { assert.Equal(t, uint64(2), r.metrics.NVerified) })
	select {
	case <-r.underreplicated:
		assert.True(t, false) // should't get message in underreplicated
	default:
	}
}

func peerMap(peerArr []peer.Peer) map[string]peer.Peer {
	peerMap := make(map[string]peer.Peer)
	for _, p := range peerArr {
		peerMap[p.ID().String()] = p
	}
	return peerMap
}

type fixedVerifier struct {
	result *verify.Result
	err    error
}

func (f *fixedVerifier) Verify(v *verify.Verify, seeds []peer.Peer) error {
	v.Result = f.result
	return f.err
}

func TestReplicator_replicate(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)
	valueBytes, err := proto.Marshal(value)
	assert.Nil(t, err)
	macKey := api.RandBytes(rng, 32)
	verifyParams := verify.NewDefaultParameters()
	r := replicator{
		selfID:          selfID,
		metrics:         &Metrics{},
		storeParams:     store.NewDefaultParameters(),
		underreplicated: make(chan *verify.Verify, 1),
		errs:            make(chan error, 1),
		logger:          zap.NewNop(), // server.NewDevLogger(zap.DebugLevel),
	}

	// start verification routine
	go r.replicate(new(sync.WaitGroup))

	// check that when storer returns result where Stored() == true, NReplicated increases
	r.storer = &fixedStorer{
		result: &store.Result{Responded: peer.NewTestPeers(rng, int(verifyParams.NReplicas))},
	}
	r.underreplicated <- verify.NewVerify(selfID, key, valueBytes, macKey, verifyParams)
	err = <-r.errs
	assert.Nil(t, err)
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NReplicated) })

	// check that when storer returns result where Stored() == false, NReplicated does not increase
	r.storer = &fixedStorer{
		result: &store.Result{Responded: []peer.Peer{}},
	}
	r.underreplicated <- verify.NewVerify(selfID, key, valueBytes, macKey, verifyParams)
	err = <-r.errs
	assert.Nil(t, err)
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NReplicated) })

	// check that when storer returns an error, it gets passed to the errs channel
	r.storer = &fixedStorer{err: errors.New("some Store error")}
	r.underreplicated <- verify.NewVerify(selfID, key, valueBytes, macKey, verifyParams)
	err = <-r.errs
	assert.NotNil(t, err)
	r.wrapLock(func() { assert.Equal(t, uint64(1), r.metrics.NReplicated) })
}

type fixedStorer struct {
	result *store.Result
	err    error
}

func (s *fixedStorer) Store(store *store.Store, seeds []peer.Peer) error {
	if s.err != nil {
		return s.err
	}
	store.Result = s.result
	return nil
}

func TestNewStore(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	doc, key := api.NewTestDocument(rng)
	value, err := proto.Marshal(doc)
	assert.Nil(t, err)
	macKey := api.RandBytes(rng, 32)

	verifyParams := &verify.Parameters{
		NReplicas:         3,
		NClosestResponses: 6,
		NMaxErrors:        3,
	}
	v := verify.NewVerify(selfID, key, value, macKey, verifyParams)

	// under-replicated by one
	responded := peer.NewTestPeers(rng, int(verifyParams.NClosestResponses))
	for _, p := range responded[:verifyParams.NReplicas-1] {
		v.Result.Replicas[p.ID().String()] = p
	}
	err = v.Result.Closest.SafePushMany(responded[verifyParams.NReplicas-1:])
	assert.Nil(t, err)
	assert.True(t, v.UnderReplicated())
	assert.False(t, v.FullyReplicated())

	s := newStore(selfID, v, *store.NewDefaultParameters())
	assert.Equal(t, verifyParams.NMaxErrors, s.Search.Params.NMaxErrors)
	assert.Equal(t, uint(1), s.Params.NReplicas)
	assert.Equal(t, uint(4), s.Search.Params.NClosestResponses)
	assert.Equal(t, v.Result.Closest, s.Search.Result.Closest)
	assert.Equal(t, v.Result.Unqueried, s.Search.Result.Unqueried)
	assert.Equal(t, v.Result.Responded, s.Search.Result.Responded)
	assert.Equal(t, v.Result.Errored, s.Search.Result.Errored)
	assert.True(t, s.Search.FoundClosestPeers())
}

func TestMetricsSL_StoreLoad_ok(t *testing.T) {
	msl := metricsSLImpl{
		inner: &storage.TestSLD{},
	}
	m1 := &Metrics{
		NVerified:        2,
		NUnderreplicated: 2,
		NReplicated:      2,
		LatestPass:       time.Now().Unix(),
	}

	err := msl.Store(m1)
	assert.Nil(t, err)

	m2, err := msl.Load()
	assert.Nil(t, err)
	assert.Equal(t, m1, m2)
}

func TestMetricsSL_Store_err(t *testing.T) {
	msl := metricsSLImpl{
		inner: &storage.TestSLD{StoreErr: errors.New("some Store error")},
	}
	m := &Metrics{
		NVerified:        2,
		NUnderreplicated: 2,
		NReplicated:      2,
		LatestPass:       time.Now().Unix(),
	}

	err := msl.Store(m)
	assert.NotNil(t, err)
}

func TestMetricsSL_Load_err(t *testing.T) {
	cases := []metricsSLImpl{
		{inner: &storage.TestSLD{LoadErr: errors.New("some Load error")}},
		{inner: &storage.TestSLD{}}, // missing doc
	}

	for _, c := range cases {
		_, err := c.Load()
		assert.NotNil(t, err)
	}
}
