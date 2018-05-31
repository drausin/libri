package comm

import (
	"math/rand"
	"testing"

	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMaybeRecordErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)
	errInvalidArg := status.Error(codes.InvalidArgument, "invalid arg")
	errInternal := status.Error(codes.Internal, "internal")

	cases := map[string]struct {
		peerID          id.ID
		err             error
		errRecorded     bool
		successRecorded bool
	}{
		"nil peer ID": {
			err:             errInternal,
			errRecorded:     false,
			successRecorded: false,
		},
		"non-grpc err": {
			peerID:          peerID,
			err:             errors.New("some other error"),
			errRecorded:     true,
			successRecorded: false,
		},
		"healthy err": {
			peerID:          peerID,
			err:             errInvalidArg,
			errRecorded:     false,
			successRecorded: true,
		},
	}
	for desc, c := range cases {
		r := &fixedRecorder{}
		MaybeRecordRpErr(r, c.peerID, api.Find, c.err)
		assert.Equal(t, c.errRecorded, r.nRecords[Error] == 1, desc)
		assert.Equal(t, c.successRecorded, r.nRecords[Success] == 1, desc)
	}
}

func TestScalarRG(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	id1, id2, id3 := id.NewPseudoRandom(rng), id.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	r := NewQueryRecorderGetter(&neverKnower{})

	// record every possible combination
	for _, e := range api.Endpoints {
		for _, qt := range []QueryType{Request, Response} {
			for _, o := range []Outcome{Success, Error} {
				r.Record(id1, e, qt, o)
			}
		}
	}

	// check that all 4 query type + outcome combos are populated for each endpoint
	for _, e := range api.Endpoints {
		eo := r.Get(id1, e)
		assert.Equal(t, uint64(1), eo[Request][Success].Count)
		assert.Equal(t, uint64(1), eo[Response][Success].Count)
		assert.Equal(t, uint64(1), eo[Request][Error].Count)
		assert.Equal(t, uint64(1), eo[Response][Error].Count)
		assert.Equal(t, 1, r.CountPeers(e, Request, false))
		assert.Equal(t, 1, r.CountPeers(e, Response, false))
	}

	// add a few responses from another peer
	r.Record(id2, api.Find, Request, Success)
	r.Record(id2, api.Store, Request, Success)

	// check p2's counts and that p1's counts remain unchanged
	assert.Equal(t, uint64(1), r.Get(id2, api.Find)[Request][Success].Count)
	assert.Equal(t, uint64(1), r.Get(id2, api.Store)[Request][Success].Count)
	for _, e := range api.Endpoints {
		eo := r.Get(id1, e)
		assert.Equal(t, uint64(1), eo[Request][Success].Count)
		assert.Equal(t, uint64(1), eo[Response][Success].Count)
		assert.Equal(t, uint64(1), eo[Request][Error].Count)
		assert.Equal(t, uint64(1), eo[Response][Error].Count)
	}

	// check endpoints w/ id1 and id2 rqs have proper peer count
	assert.Equal(t, 2, r.CountPeers(api.Find, Request, false))
	assert.Equal(t, 2, r.CountPeers(api.Store, Request, false))
	assert.Equal(t, 1, r.CountPeers(api.Find, Response, false))
	assert.Equal(t, 1, r.CountPeers(api.Store, Response, false))

	// check p3's counts are zero, since we haven't recorded anything for it yet
	assert.Equal(t, uint64(0), r.Get(id3, api.Find)[Request][Success].Count)
	assert.Equal(t, uint64(0), r.Get(id3, api.Store)[Request][Success].Count)
}

func TestWindowRG(t *testing.T) {
	k := &neverKnower{}
	window := 50 * time.Millisecond
	r := NewWindowRecorderGetter(k, window)
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)

	r.Record(peerID, api.Find, Request, Success)
	assert.Equal(t, uint64(1), r.Get(peerID, api.Find)[Request][Success].Count)
	assert.Equal(t, 1, r.CountPeers(api.Find, Request, false))

	time.Sleep(window)

	// counts should no longer exist
	assert.Equal(t, uint64(0), r.Get(peerID, api.Find)[Request][Success].Count)
	assert.Equal(t, 0, r.CountPeers(api.Find, Request, false))
}

func TestWindowQueryRecorders_Record(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)
	secR, dayR := &fixedRecorder{}, &fixedRecorder{}
	wqrs := WindowQueryRecorders{
		Second: secR,
		Day:    dayR,
	}
	wqrs.Record(peerID, api.Find, Response, Success)

	// check each duration has record
	assert.Equal(t, 1, secR.nRecords[Success])
	assert.Equal(t, 1, dayR.nRecords[Success])
}

func TestPromScalarRecorder_Record(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := id.NewPseudoRandom(rng)
	id1, id2 := id.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	qr := &fixedRecorder{}

	r := NewPromScalarRecorder(selfID, qr)

	r.Record(id1, api.Find, Request, Success)
	r.Record(id1, api.Store, Request, Success)
	r.Record(id2, api.Store, Response, Success)

	metrics := make(chan prom.Metric, 4)
	r.(*promQR).counter.Collect(metrics)
	close(metrics)
	for m := range metrics {
		written := &dto.Metric{}
		m.Write(written)
		assert.Equal(t, float64(1), *written.Counter.Value)
		assert.Equal(t, 5, len(written.Label))
	}
}
