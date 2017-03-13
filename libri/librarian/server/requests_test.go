package server

import (
	"crypto/ecdsa"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/golang/protobuf/proto"
)

// alwaysSigVerifier implements the signature.Verifier interface but just blindly verifies
// every signature.
type alwaysSigVerifier struct{}

func (asv *alwaysSigVerifier) Verify(encToken string, fromPubKey *ecdsa.PublicKey,
	m proto.Message) error {
	return nil
}

func TestRequestVerifier_Verify_ok(t *testing.T) {
	rv := &verifier{
		sigVerifier: &alwaysSigVerifier{},
	}

	rng := rand.New(rand.NewSource(0))
	ctx := client.NewSignatureContext(context.Background(), "dummy.signed.token")
	meta := client.NewRequestMetadata(ecid.NewPseudoRandom(rng))

	assert.Nil(t, rv.Verify(ctx, nil, meta))
}

func TestRequestVerifier_Verify_err(t *testing.T) {
	rv := &verifier{
		sigVerifier: &alwaysSigVerifier{},
	}

	assert.NotNil(t, rv.Verify(context.Background(), nil, nil))  // no signature in context

	rng := rand.New(rand.NewSource(0))
	ctx := client.NewSignatureContext(context.Background(), "dummy.signed.token")

	assert.NotNil(t, rv.Verify(ctx, nil, &api.RequestMetadata{
		PubKey: []byte{255, 254, 253}, // bad pub key
	}))

	assert.NotNil(t, rv.Verify(ctx, nil, &api.RequestMetadata{
		PubKey:    ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng)),
		RequestId: nil, // can't be nil
	}))

	assert.NotNil(t, rv.Verify(ctx, nil, &api.RequestMetadata{
		PubKey:    ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng)),
		RequestId: []byte{1, 2, 3}, // not 32 bytes
	}))
}
