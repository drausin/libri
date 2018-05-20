package comm

import (
	"testing"

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
	qo1 := endpointQueryOutcomes{
		api.Find: QueryOutcomes{
			Request: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
		},
	}
	qo2 := endpointQueryOutcomes{
		api.Find: {
			Request: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 2},
			},
		},
	}
	qo3 := endpointQueryOutcomes{
		api.Find: {
			Request: map[Outcome]*ScalarMetrics{
				Success: {Count: 1},
			},
			Response: map[Outcome]*ScalarMetrics{
				Success: {Count: 3},
			},
		},
	}

	rec := &scalarRG{
		peers: map[string]endpointQueryOutcomes{
			peerID1.String(): qo1,
			peerID2.String(): qo2,
			peerID3.String(): qo3,
		},
	}
	p := NewFindRpPreferer(rec)

	// prefer peer w/ more successful Find responses
	assert.True(t, p.Prefer(peerID2, peerID1))
	assert.True(t, p.Prefer(peerID3, peerID2))
}
