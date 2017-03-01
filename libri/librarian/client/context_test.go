package client

import (
	"math/rand"
	"testing"
	"time"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/stretchr/testify/assert"
)

func TestNewSignedTimeoutContext_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	ctx, cancel, err := NewSignedTimeoutContext(
		&signature.TestNoOpSigner{},
		api.NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), 20),
		5*time.Second,
	)
	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.Value(signature.NewContextKey()))
	assert.NotNil(t, cancel)
	assert.Nil(t, err)
}

func TestNewSignedTimeoutContext_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	ctx, cancel, err := NewSignedTimeoutContext(
		&signature.TestErrSigner{},
		api.NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), 20),
		5*time.Second,
	)
	assert.Nil(t, ctx)
	assert.NotNil(t, cancel)
	assert.NotNil(t, err)
}
