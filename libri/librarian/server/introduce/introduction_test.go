package introduce

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"errors"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultParameters(t *testing.T) {
	params := NewDefaultParameters()
	assert.NotNil(t, params.TargetNumIntroductions)
	assert.NotNil(t, params.NumPeersPerRequest)
	assert.NotNil(t, params.NMaxErrors)
	assert.NotNil(t, params.Concurrency)
	assert.NotNil(t, params.Timeout)
}

func TestIntroduction_ReachedTarget(t *testing.T) {
	intro := newTestIntroduction(3, &Parameters{
		TargetNumIntroductions: 3,
		NMaxErrors:             3,
	})

	// hasn't received any responses yet
	assert.False(t, intro.ReachedTarget())
	assert.False(t, intro.Errored())
	assert.False(t, intro.Exhausted())
	assert.False(t, intro.Finished())

	// make some peers responded, but still below target
	simulateAnyResponded(intro.Result, 2)

	// still below target
	assert.False(t, intro.ReachedTarget())
	assert.False(t, intro.Errored())
	assert.False(t, intro.Exhausted())
	assert.False(t, intro.Finished())

	// add another to get us to target
	simulateAnyResponded(intro.Result, 1)

	// now should register target as reached
	assert.True(t, intro.ReachedTarget())
	assert.False(t, intro.Errored())
	assert.False(t, intro.Exhausted())
	assert.True(t, intro.Finished())
}

func TestIntroduction_Exhausted(t *testing.T) {
	intro := newTestIntroduction(4, &Parameters{
		TargetNumIntroductions: 3,
		NMaxErrors:             3,
	})

	// make some peers responded, but still below target
	simulateAnyResponded(intro.Result, 2)

	// still have some unqueried peers
	assert.False(t, intro.Exhausted())
	assert.False(t, intro.ReachedTarget())
	assert.False(t, intro.Errored())
	assert.False(t, intro.Finished())

	// make a peer not respond
	simulateAnyNotResponded(intro.Result, 1)

	// still have unqueried peers though
	assert.False(t, intro.Exhausted())
	assert.False(t, intro.ReachedTarget())
	assert.False(t, intro.Errored())
	assert.False(t, intro.Finished())

	// make last peer also not responded
	simulateAnyNotResponded(intro.Result, 1)
	assert.True(t, intro.Exhausted())
	assert.False(t, intro.ReachedTarget())
	assert.False(t, intro.Errored())
	assert.True(t, intro.Finished())
}

func TestIntroduction_Errored(t *testing.T) {
	intro1 := newTestIntroduction(4, &Parameters{
		TargetNumIntroductions: 3,
		NMaxErrors:             3,
	})

	// add some errors, but still below NMaxErrors
	intro1.Result.NErrors += 2
	assert.False(t, intro1.Errored())
	assert.False(t, intro1.Exhausted())
	assert.False(t, intro1.ReachedTarget())
	assert.False(t, intro1.Finished())

	// add another error, putting us over the top
	intro1.Result.NErrors++
	assert.True(t, intro1.Errored())
	assert.False(t, intro1.Exhausted())
	assert.False(t, intro1.ReachedTarget())
	assert.True(t, intro1.Finished())

	intro2 := newTestIntroduction(4, &Parameters{
		NMaxErrors: 3,
	})

	// create some fatal error
	intro2.Result.FatalErr = errors.New("some fata error")
	assert.True(t, intro1.Errored())
	assert.False(t, intro1.Exhausted())
	assert.False(t, intro1.ReachedTarget())
	assert.True(t, intro1.Finished())
}

func newTestIntroduction(nPeers int, params *Parameters) *Introduction {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)
	self := peer.New(selfID.ID(), "test peer", peer.NewTestConnector(nPeers+1))
	intro := NewIntroduction(selfID, self.ToAPI(), params)

	// add some peers to unqueried map
	for _, p := range peer.NewTestPeers(rng, nPeers) {
		intro.Result.Unqueried[p.ID().String()] = p
	}
	return intro
}

func simulateAnyResponded(result *Result, n int) {
	c := 0
	for k, v := range result.Unqueried {
		result.Responded[k] = v
		delete(result.Unqueried, k)
		c++
		if c == n {
			break
		}
	}
}

func simulateAnyNotResponded(result *Result, n int) {
	c := 0
	for k := range result.Unqueried {
		delete(result.Unqueried, k)
		c++
		if c == n {
			break
		}
	}
}
