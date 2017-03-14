package client

import (
	"math/rand"
	"testing"
	"time"

	"context"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestNewFromSignatureContext(t *testing.T) {
	ctx := context.Background()
	signedToken1 := "some.signed.token"
	signedCtx := NewSignatureContext(ctx, signedToken1)
	signedToken2, err := FromSignatureContext(signedCtx)
	assert.Equal(t, signedToken1, signedToken2)
	assert.Nil(t, err)
}

func TestFromSignatureContext_missingMetadataErr(t *testing.T) {
	signedToken, err := FromSignatureContext(context.Background())
	assert.Zero(t, signedToken)
	assert.NotNil(t, err)
}

func TestFromSignatureContext_missingSignatureErr(t *testing.T) {
	ctx := metadata.NewContext(context.Background(), metadata.MD{}) // no signature key
	signedToken, err := FromSignatureContext(ctx)
	assert.Zero(t, signedToken)
	assert.NotNil(t, err)
}
func TestNewSignedTimeoutContext_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	ctx, cancel, err := NewSignedTimeoutContext(
		&TestNoOpSigner{},
		NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), 20),
		5*time.Second,
	)
	assert.NotNil(t, ctx)

	md, in := metadata.FromContext(ctx)
	assert.True(t, in)
	assert.NotNil(t, md[signatureKey])
	assert.NotNil(t, cancel)
	assert.Nil(t, err)
}

func TestNewSignedTimeoutContext_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	ctx, cancel, err := NewSignedTimeoutContext(
		&TestErrSigner{},
		NewFindRequest(ecid.NewPseudoRandom(rng), cid.NewPseudoRandom(rng), 20),
		5*time.Second,
	)
	assert.Nil(t, ctx)
	assert.NotNil(t, cancel)
	assert.NotNil(t, err)
}
