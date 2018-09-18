package comm

import (
	"testing"

	"math/rand"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestRpPreferer_Prefer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	peerID1 := id.NewPseudoRandom(rng)
	peerID2 := id.NewPseudoRandom(rng)
	peerID3 := id.NewPseudoRandom(rng)
	peerID4 := id.NewPseudoRandom(rng)
	qo1 := endpointQueryOutcomes{
		api.Verify: QueryOutcomes{
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
		},
		api.Find: QueryOutcomes{
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
		},
	}
	qo2 := endpointQueryOutcomes{
		api.Verify: {
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
		},
		api.Find: QueryOutcomes{
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
		},
	}
	qo3 := endpointQueryOutcomes{
		api.Verify: {
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 3},
			},
		},
		api.Find: QueryOutcomes{
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 3},
			},
		},
	}
	qo4 := endpointQueryOutcomes{
		api.Verify: newQueryOutcomes(),
		api.Find: QueryOutcomes{
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 5},
			},
		},
	}

	rec := &scalarRG{
		peers: map[string]endpointQueryOutcomes{
			peerID1.String(): qo1,
			peerID2.String(): qo2,
			peerID3.String(): qo3,
			peerID4.String(): qo4,
		},
	}
	p := NewRpPreferer(rec)

	// prefer peer w/ more successful Verify or Find responses
	assert.True(t, p.Prefer(peerID2, peerID1))
	assert.True(t, p.Prefer(peerID3, peerID2))
	assert.True(t, p.Prefer(peerID4, peerID3))
}
