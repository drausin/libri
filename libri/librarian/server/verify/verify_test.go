package verify

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewDefaultParameters(t *testing.T) {
	p := NewDefaultParameters()
	assert.NotZero(t, p.NReplicas)
	assert.NotZero(t, p.NClosestResponses)
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
	rng := rand.New(rand.NewSource(0))
	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())
	r := NewInitialResult(id.NewPseudoRandom(rng), NewDefaultParameters())
	r.Errored["some peer ID"] = errors.New("some error")
	r.FatalErr = errors.New("some fatal error")
	err := r.MarshalLogObject(oe)
	assert.Nil(t, err)
}

func TestVerify_MarshalLogObject(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())
	s := NewVerify(
		ecid.NewPseudoRandom(rng),
		id.NewPseudoRandom(rng),
		[]byte{1, 2, 3},
		[]byte{4, 5, 6},
		NewDefaultParameters(),
	)
	err := s.MarshalLogObject(oe)
	assert.Nil(t, err)
}

func TestVerify_PartiallyReplicated(t *testing.T) {
	// target = 0 makes it easy to compute XOR distance manually
	rng := rand.New(rand.NewSource(0))
	target, selfID := id.FromInt64(0), ecid.NewPseudoRandom(rng)
	macKey, mac := []byte{1, 2, 3}, []byte{4, 5, 6}

	v := NewVerify(selfID, target, macKey, mac, &Parameters{
		NReplicas:         3,
		NClosestResponses: 6,
		Concurrency:       3,
	})

	// add closest peers to half the heap's capacity
	err := v.Result.Closest.SafePushMany([]peer.Peer{
		peer.New(id.FromInt64(1), "", nil),
		peer.New(id.FromInt64(2), "", nil),
	})
	assert.Nil(t, err)

	// not partially replicated b/c closest heap not at capacity
	assert.False(t, v.UnderReplicated())

	// add an unqueried peer farther than the farthest closest peer
	err = v.Result.Unqueried.SafePush(peer.New(id.FromInt64(7), "", nil))
	assert.Nil(t, err)
	assert.Equal(t, 1, v.Result.Unqueried.Len())

	// still not partially replicated b/c closest heap not at capacity
	assert.False(t, v.UnderReplicated())

	// add two replicated & two close peers, bringing total replicas + closest above number of
	// closest responses
	v.Result.Replicas = map[string]peer.Peer{
		"3": peer.New(id.FromInt64(3), "", nil),
		"4": peer.New(id.FromInt64(4), "", nil),
	}
	err = v.Result.Closest.SafePushMany([]peer.Peer{
		peer.New(id.FromInt64(5), "", nil),
		peer.New(id.FromInt64(6), "", nil),
	})
	assert.Nil(t, err)

	// now that closest peers is at capacity, and it's max distance is less than the min
	// unqueried peers distance, verify is now partially replicated
	assert.True(t, v.UnderReplicated())
	assert.False(t, v.Exhausted())

	// add another replica, making it fully (and no longer partially) replicated
	v.Result.Replicas["8"] = peer.New(id.FromInt64(8), "", nil)
	assert.True(t, v.FullyReplicated())
	assert.False(t, v.UnderReplicated())
	assert.False(t, v.Exhausted())
}

func TestVerify_FullyReplicated(t *testing.T) {
	// target = 0 makes it easy to compute XOR distance manually
	rng := rand.New(rand.NewSource(0))
	target, selfID := id.FromInt64(0), ecid.NewPseudoRandom(rng)
	macKey, mac := []byte{1, 2, 3}, []byte{4, 5, 6}

	v := NewVerify(selfID, target, macKey, mac, &Parameters{
		NReplicas:         3,
		NClosestResponses: 6,
		Concurrency:       3,
	})

	// add some replicas (but not enough for full replication)
	v.Result.Replicas = map[string]peer.Peer{
		"1": peer.New(id.FromInt64(1), "", nil),
		"2": peer.New(id.FromInt64(2), "", nil),
	}
	assert.False(t, v.FullyReplicated())

	// add another replica, bringing us up to full replication
	v.Result.Replicas["3"] = peer.New(id.FromInt64(3), "", nil)
	assert.True(t, v.FullyReplicated())
	assert.False(t, v.Exhausted())
}

func TestVerify_Errored(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	target, selfID := id.FromInt64(0), ecid.NewPseudoRandom(rng)
	macKey, mac := []byte{1, 2, 3}, []byte{4, 5, 6}

	// no-error state
	search1 := NewVerify(selfID, target, macKey, mac, NewDefaultParameters())
	assert.False(t, search1.Errored())

	// errored state b/c of too many errors
	search2 := NewVerify(selfID, target, macKey, mac, NewDefaultParameters())
	for c := uint(0); c < search2.Params.NMaxErrors+1; c++ {
		peerID := id.NewPseudoRandom(rng).String()
		search2.Result.Errored[peerID] = errors.New("some Find error")
	}
	assert.True(t, search2.Errored())

	// errored state b/c of a fatal error
	search3 := NewVerify(selfID, target, macKey, mac, NewDefaultParameters())
	search3.Result.FatalErr = errors.New("test fatal error")
	assert.True(t, search3.Errored())
}

func TestVerify_Exhausted(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	target, selfID := id.FromInt64(0), ecid.NewPseudoRandom(rng)
	macKey, mac := []byte{1, 2, 3}, []byte{4, 5, 6}

	// not exhausted b/c it has unqueried peers
	search1 := NewVerify(selfID, target, macKey, mac, NewDefaultParameters())
	err := search1.Result.Unqueried.SafePush(peer.New(id.FromInt64(1), "", nil))
	assert.Nil(t, err)
	assert.False(t, search1.Exhausted())

	// exhausted b/c it doesn't have unqueried peers
	search2 := NewVerify(selfID, target, macKey, mac, NewDefaultParameters())
	err = search2.Result.Unqueried.SafePush(peer.New(id.FromInt64(1), "", nil))
	assert.Nil(t, err)
	assert.False(t, search1.Exhausted())
}
