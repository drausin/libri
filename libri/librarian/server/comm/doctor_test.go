package comm

import (
	"math/rand"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestNaiveDoctor_Healthy(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	d := NewNaiveDoctor()
	assert.True(t, d.Healthy(id.NewPseudoRandom(rng)))
}

func TestResponseTimeDoctor_Healthy(t *testing.T) {
	now := time.Now()
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)

	// check not healthy since latest success was 15 mins before latest error
	qo1 := newQueryOutcomes()
	qo1[Response][Error].Latest = now
	qo1[Response][Success].Latest = now.Add(-15 * time.Minute)
	d1 := NewResponseTimeDoctor(&fixedRecorder{getValue: qo1})
	assert.False(t, d1.Healthy(peerID))

	// check not healthy since no verifications and latest Find success 15 mins before latest
	// error
	g4 := &fixedGetter{
		outcomes: endpointQueryOutcomes{
			api.Verify: newQueryOutcomes(),
			api.Find:   newQueryOutcomes(),
		},
	}
	g4.outcomes[api.Find][Response][Error].Latest = now
	g4.outcomes[api.Find][Response][Success].Latest = now.Add(-15 * time.Minute)
	d4 := NewResponseTimeDoctor(g4)
	assert.False(t, d4.Healthy(peerID))

	// check healthy since latest success only 3 mins before latest error
	qo2 := newQueryOutcomes()
	qo2[Response][Error].Latest = now
	qo2[Response][Success].Latest = now.Add(-3 * time.Minute)
	d2 := NewResponseTimeDoctor(&fixedRecorder{getValue: qo2})
	assert.True(t, d2.Healthy(peerID))

	// check healthy since latest success was 5 mins after latest error
	qo3 := newQueryOutcomes()
	qo3[Response][Error].Latest = now
	qo3[Response][Success].Latest = now.Add(5 * time.Minute)
	d3 := NewResponseTimeDoctor(&fixedRecorder{getValue: qo3})
	assert.True(t, d3.Healthy(peerID))
}

type fixedGetter struct {
	outcomes endpointQueryOutcomes
}

func (f *fixedGetter) Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes {
	return f.outcomes[endpoint]
}

func (f *fixedGetter) CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int {
	panic("implement me")
}
