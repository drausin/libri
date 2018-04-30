package goodwill

import (
	"math/rand"
	"testing"

	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestLatestCompareJudge_Prefer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	r := NewScalarRecorder().(*scalarRecorder)
	j := NewLatestPreferJudge(r)

	id1, id2 := id.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	t1 := time.Unix(1524017559, 0)
	t2 := t1.Add(-5 * time.Minute)
	t3 := t1.Add(-5 * time.Second)

	// prefer 1 over 2 b/c it had a successful response more recently
	r.peers[id1.String()] = singletonResponseSuccess(t1, 1)
	r.peers[id2.String()] = singletonResponseSuccess(t2, 1)
	assert.True(t, j.Prefer(id1, id2))

	// same, even when 2 has larger count
	r.peers[id1.String()] = singletonResponseSuccess(t1, 1)
	r.peers[id2.String()] = singletonResponseSuccess(t2, 2)
	assert.True(t, j.Prefer(id1, id2))

	// prefer 2 over 1 b/c it's within same minute window and we've made fewer queries to it
	r.peers[id1.String()] = singletonResponseSuccess(t1, 2)
	r.peers[id2.String()] = singletonResponseSuccess(t3, 1)
	assert.False(t, j.Prefer(id1, id2))

}

func singletonResponseSuccess(latest time.Time, count uint64) EndpointQueryOutcomes {
	return EndpointQueryOutcomes{
		api.Find: QueryOutcomes{
			Response: map[Outcome]*ScalarMetrics{
				Success: {
					Latest: latest,
					Count:  count,
				},
				Error: {},
			},
			Request: map[Outcome]*ScalarMetrics{
				Success: {},
				Error:   {},
			},
		},
	}
}
