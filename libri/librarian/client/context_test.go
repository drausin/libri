package client

import (
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/librarian/signature"
	"testing"
	"math/rand"
	"github.com/drausin/libri/libri/librarian/api"
	cid "github.com/drausin/libri/libri/common/id"
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

