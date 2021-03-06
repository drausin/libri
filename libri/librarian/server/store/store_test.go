package store

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewInitialResult(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	n := 6
	sr := &ssearch.Result{
		Closest: ssearch.NewFarthestPeers(target, uint(n)),
	}
	for _, p := range peer.NewTestPeers(rng, n) {
		sr.Closest.SafePush(p)
	}

	ir := NewInitialResult(sr)
	assert.Len(t, ir.Unqueried, n)
	assert.Len(t, ir.Responded, 0)
	assert.NotNil(t, ir.Search)
	assert.Len(t, ir.Errors, 0)

	// check unqueried queue is ordered from closest-to-farthest
	for i := 1; i < len(ir.Unqueried); i++ {
		p1, p2 := ir.Unqueried[i-1], ir.Unqueried[i]
		assert.NotNil(t, p1)
		assert.NotNil(t, p2)
		dist1, dist2 := target.Distance(p1.ID()), target.Distance(p2.ID())
		assert.True(t, dist1.Cmp(dist2) < 0)
	}
}

func TestNewDefaultParameters(t *testing.T) {
	p := NewDefaultParameters()
	assert.NotZero(t, p.NMaxErrors)
	assert.NotZero(t, p.Concurrency)
	assert.NotZero(t, p.Timeout)
}

func TestParameters_MarshalLogObject(t *testing.T) {
	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())

	p := NewDefaultParameters()
	err := p.MarshalLogObject(oe)
	assert.Nil(t, err)
}

func TestResult_MarshalLogObject(t *testing.T) {
	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())

	var r1 *Result
	err := r1.MarshalLogObject(oe)
	assert.Nil(t, err)

	r2 := NewFatalResult(errors.New("some fatal error"))
	r2.Errors = []error{errors.New("some non-fatal error")}
	err = r2.MarshalLogObject(oe)
	assert.Nil(t, err)
}

func TestStore_MarshalLogObject_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)

	var s1 *Store
	err := s1.MarshalLogObject(nil)
	assert.Nil(t, err)

	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())
	doc, key := api.NewTestDocument(rng)
	searchParams := ssearch.NewDefaultParameters()
	s2 := NewStore(peerID, orgID, key, doc, searchParams, NewDefaultParameters())
	s2.Result = NewInitialResult(ssearch.NewInitialResult(key, searchParams))
	err = s2.MarshalLogObject(oe)
	assert.Nil(t, err)
}

func TestStore_Stored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)

	// create store with search
	store := NewStore(peerID, orgID, key, value, &ssearch.Parameters{}, &Parameters{
		NReplicas:  3,
		NMaxErrors: 3,
	})
	store.Result = NewInitialResult(store.Search.Result)
	store.Result.Unqueried = []peer.Peer{nil} // just needs to be non-zero length

	// not stored yet b/c have no peers that have responded to store query
	assert.False(t, store.Stored())
	assert.False(t, store.Finished())

	// add a few errors, still not stored
	store.Result.Errors = []error{errors.New("1"), errors.New("2")}
	assert.False(t, store.Stored())
	assert.False(t, store.Finished())

	// once we receive responses from enough peers (here, faked w/ nils), it's stored
	store.Result.Responded = append(store.Result.Responded, nil, nil, nil)
	assert.True(t, store.Stored())
	assert.True(t, store.Finished())
}

func TestStore_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	orgID := ecid.NewPseudoRandom(rng)

	// create search with result of closest peers
	nClosestResponse := uint(4)
	search := ssearch.NewSearch(peerID, orgID, key, &ssearch.Parameters{
		NClosestResponses: nClosestResponse,
	})

	// create new store
	searchResult := ssearch.NewInitialResult(key, &ssearch.Parameters{
		NMaxErrors: 3,
	})
	s := &Store{
		Params: &Parameters{
			NReplicas:  3,
			NMaxErrors: 3,
		},
		Result: NewInitialResult(searchResult),
		Search: search,
	}
	s.Result.Unqueried = []peer.Peer{nil} // just needs to be non-zero length

	// haven't received any errors yet
	assert.False(t, s.Errored())
	assert.False(t, s.Finished())

	// add errors, but not too many
	s.Result.Errors = append(s.Result.Errors, errors.New("1"), errors.New("2"))

	// still within acceptable range of errors
	assert.False(t, s.Errored())
	assert.False(t, s.Finished())

	// push over the edge
	s.Result.Errors = append(s.Result.Errors, errors.New("3"))
	assert.True(t, s.Errored())
	assert.True(t, s.Finished())

	// or, if we receive a fatal error
	s.Result.Errors = []error{}
	s.Result.FatalErr = errors.New("some fatal error")
	assert.True(t, s.Errored())
	assert.True(t, s.Finished())
}
