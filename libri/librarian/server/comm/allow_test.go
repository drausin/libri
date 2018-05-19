package comm

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errTest = errors.New("test error")
)

func TestAllower_Allow(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)
	cases := map[string]struct {
		allower         Allower
		expectedErrCode codes.Code
	}{
		"ok": {
			allower: NewAllower(
				&fixedAuthorizer{},
				&fixedLimiter{},
				&fixedLimiter{},
			),
			expectedErrCode: codes.OK,
		},
		"not authorized": {
			allower: NewAllower(
				&fixedAuthorizer{err: errTest},
				&fixedLimiter{},
				&fixedLimiter{},
			),
			expectedErrCode: codes.PermissionDenied,
		},
		"peer limited": {
			allower: NewAllower(
				&fixedAuthorizer{},
				&fixedLimiter{err: errTest},
				&fixedLimiter{},
			),
			expectedErrCode: codes.ResourceExhausted,
		},
		"query limited": {
			allower: NewAllower(
				&fixedAuthorizer{},
				&fixedLimiter{},
				&fixedLimiter{err: errTest},
			),
			expectedErrCode: codes.ResourceExhausted,
		},
	}
	for desc, c := range cases {
		err := c.allower.Allow(peerID, api.Find)
		errSt, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, c.expectedErrCode, errSt.Code(), desc)
	}
}

func TestAlwaysAuthorizer_Authorized(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)
	a := NewAlwaysAuthorizer()
	assert.Nil(t, a.Authorized(peerID, api.Find))
}

func TestPeerLimiter_WithinLimit(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)
	cases := map[string]struct {
		lim      Limiter
		endpoint api.Endpoint
		expected error
	}{
		"no limits": {
			lim: NewPeerLimiter(
				Limits{},
				&neverKnower{},
				&fixedRecorder{countValue: 1},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"no endpoint limit": {
			lim: NewPeerLimiter(
				Limits{api.Store: {false: 2}},
				&neverKnower{},
				&fixedRecorder{countValue: 3},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"no unknown limit": {
			lim: NewPeerLimiter(
				Limits{api.Find: {true: 2}},
				&neverKnower{},
				&fixedRecorder{countValue: 1},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"within limit": {
			lim: NewPeerLimiter(
				Limits{api.Find: {false: 2}},
				&neverKnower{},
				&fixedRecorder{countValue: 1},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"known, above limit": {
			lim: NewPeerLimiter(
				Limits{api.Find: {true: 2}},
				&friendlyKnower{},
				&fixedRecorder{countValue: 3},
			),
			endpoint: api.Find,
			expected: ErrKnownAbovePeerLimit,
		},
		"unknown, above limit": {
			lim: NewPeerLimiter(
				Limits{api.Find: {false: 2}},
				&neverKnower{},
				&fixedRecorder{countValue: 3},
			),
			endpoint: api.Find,
			expected: ErrUnknownAbovePeerLimit,
		},
	}
	for desc, c := range cases {
		err := c.lim.WithinLimit(peerID, c.endpoint)
		assert.Equal(t, err, c.expected, desc)
	}
}

func TestQueryLimiter_WithinLimit(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	peerID := id.NewPseudoRandom(rng)
	cases := map[string]struct {
		lim      Limiter
		endpoint api.Endpoint
		expected error
	}{
		"no limits": {
			lim: NewQueryLimiter(
				Limits{},
				&neverKnower{},
				&fixedRecorder{getValue: newRqSuccessCount(1)},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"no endpoint limit": {
			lim: NewQueryLimiter(
				Limits{api.Store: {false: 2}},
				&neverKnower{},
				&fixedRecorder{getValue: newRqSuccessCount(3)},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"no unknown limit": {
			lim: NewQueryLimiter(
				Limits{api.Find: {true: 2}},
				&neverKnower{},
				&fixedRecorder{getValue: newRqSuccessCount(1)},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"within limit": {
			lim: NewQueryLimiter(
				Limits{api.Find: {false: 2}},
				&neverKnower{},
				&fixedRecorder{getValue: newRqSuccessCount(1)},
			),
			endpoint: api.Find,
			expected: nil,
		},
		"known, above limit": {
			lim: NewQueryLimiter(
				Limits{api.Find: {true: 2}},
				&friendlyKnower{},
				&fixedRecorder{getValue: newRqSuccessCount(3)},
			),
			endpoint: api.Find,
			expected: ErrKnownAboveQueryLimit,
		},
		"unknown, above limit": {
			lim: NewQueryLimiter(
				Limits{api.Find: {false: 2}},
				&neverKnower{},
				&fixedRecorder{getValue: newRqSuccessCount(3)},
			),
			endpoint: api.Find,
			expected: ErrUnknownAboveQueryLimit,
		},
	}
	for desc, c := range cases {
		err := c.lim.WithinLimit(peerID, c.endpoint)
		assert.Equal(t, err, c.expected, desc)
	}
}

func newRqSuccessCount(count uint64) QueryOutcomes {
	return QueryOutcomes{
		Request: {
			Success: {
				Count: count,
			},
			Error: {},
		},
	}
}

type fixedRecorder struct {
	getValue   QueryOutcomes
	countValue int
}

func (f *fixedRecorder) Record(peerID id.ID, endpoint api.Endpoint, qt QueryType, o Outcome) {}

func (f *fixedRecorder) Get(peerID id.ID, endpoint api.Endpoint) QueryOutcomes {
	return f.getValue
}

func (f *fixedRecorder) CountPeers(endpoint api.Endpoint, qt QueryType, known bool) int {
	return f.countValue
}

type friendlyKnower struct{}

func (k *friendlyKnower) Know(peerID id.ID) bool {
	return true
}

type fixedLimiter struct {
	err error
}

func (f *fixedLimiter) WithinLimit(peerID id.ID, endpoint api.Endpoint) error {
	return f.err
}

type fixedAuthorizer struct {
	err error
}

func (f *fixedAuthorizer) Authorized(peerID id.ID, endpoint api.Endpoint) error {
	return f.err
}

type neverKnower struct{}

func (k *neverKnower) Know(peerID id.ID) bool {
	return false
}
