package goodwill

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestScalarRecorder_RecordGet(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	id1, id2, id3 := id.NewPseudoRandom(rng), id.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	r := NewScalarRecorder()

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

	// check p3's counts are zero, since we haven't recorded anything for it yet
	assert.Equal(t, uint64(0), r.Get(id3, api.Find)[Request][Success].Count)
	assert.Equal(t, uint64(0), r.Get(id3, api.Store)[Request][Success].Count)
}

func TestPromScalarRecorder_Record(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := id.NewPseudoRandom(rng)
	id1, id2 := id.NewPseudoRandom(rng), id.NewPseudoRandom(rng)

	r := NewPromScalarRecorder(selfID)

	r.Record(id1, api.Find, Request, Success)
	r.Record(id1, api.Store, Request, Success)
	r.Record(id2, api.Store, Response, Success)

	metrics := make(chan prom.Metric, 4)
	r.(*promScalarRecorder).counter.Collect(metrics)
	close(metrics)
	for m := range metrics {
		written := &dto.Metric{}
		m.Write(written)
		assert.Equal(t, float64(1), *written.Counter.Value)
		assert.Equal(t, 5, len(written.Label))
	}
}
