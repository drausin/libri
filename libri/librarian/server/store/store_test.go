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
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap"
)

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

	var s1 *Store
	err := s1.MarshalLogObject(nil)
	assert.Nil(t, err)

	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())
	doc, key := api.NewTestDocument(rng)
	searchParams := ssearch.NewDefaultParameters()
	s2 := NewStore(ecid.NewPseudoRandom(rng), key, doc, searchParams, NewDefaultParameters())
	s2.Result = NewInitialResult(ssearch.NewInitialResult(key, searchParams))
	err = s2.MarshalLogObject(oe)
	assert.Nil(t, err)
}

func TestStore_Stored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := ecid.NewPseudoRandom(rng)
	value, key := api.NewTestDocument(rng)

	// create store with search
	store := NewStore(peerID, key, value, &ssearch.Parameters{}, &Parameters{
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

	// create search with result of closest peers
	nClosestResponse := uint(4)
	search := ssearch.NewSearch(peerID, key, &ssearch.Parameters{
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
