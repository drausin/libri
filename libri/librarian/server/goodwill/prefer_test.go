package goodwill

import (
	"testing"
	"time"

	"math/rand"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestFindRqRpBetaPreferer_Prefer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	peerID1 := id.NewPseudoRandom(rng)
	peerID2 := id.NewPseudoRandom(rng)
	peerID3 := id.NewPseudoRandom(rng)
	qo1 := EndpointQueryOutcomes{
		api.Find: QueryOutcomes{
			Request: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
		},
	}
	qo2 := EndpointQueryOutcomes{
		api.Find: {
			Request: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
		},
	}
	qo3 := EndpointQueryOutcomes{
		api.Find: {
			Request: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 3},
			},
		},
	}

	rec := &scalarRecorder{
		peers: map[string]EndpointQueryOutcomes{
			peerID1.String(): qo1,
			peerID2.String(): qo2,
			peerID3.String(): qo3,
		},
	}
	p := NewDefaultFindRqRpBetaPreferer(rec)

	// both peer 2 & peer 1 have posterior mean 0, so peer 2 isn't preferred over peer 1
	assert.False(t, p.Prefer(peerID2, peerID1))

	// peer 1 has posterior mean 0, which has max log prob under ideal posterior
	assert.True(t, p.Prefer(peerID1, peerID3))
}

func TestCachedValue_get(t *testing.T) {
	cv := &cachedValue{
		calc:     func() float64 { return 2 },
		lastCalc: time.Now().Add(-2 * defaultCacheTTL), // TTL expired
		ttl:      defaultCacheTTL,
	}
	assert.Equal(t, float64(2), cv.get()) // calcs

	cv.calc = func() float64 { return 3 }
	assert.Equal(t, float64(2), cv.get()) // uses cache

	cv.lastCalc = time.Now().Add(-2 * defaultCacheTTL) // TTL expired again
	assert.Equal(t, float64(3), cv.get())              // calcs
}
