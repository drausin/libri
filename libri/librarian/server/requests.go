package server

import (
	"fmt"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/golang/protobuf/proto"
	"errors"
	"golang.org/x/net/context"
)

// RequestVerifier verifies requests by checking the signature in the context.
type RequestVerifier interface {
	Verify(ctx context.Context, msg proto.Message, meta *api.RequestMetadata) error
}

type verifier struct {
	sigVerifier client.Verifier
}

// NewRequestVerifier creates a new RequestVerifier instance.
func NewRequestVerifier() RequestVerifier {
	return &verifier{
		sigVerifier: client.NewVerifier(),
	}
}

func (rv *verifier) Verify(ctx context.Context, msg proto.Message,
	meta *api.RequestMetadata) error {
	encToken, err := client.FromSignatureContext(ctx)
	if err != nil {
		return err
	}
	pubKey, err := ecid.FromPublicKeyBytes(meta.PubKey)
	if err != nil {
		return err
	}
	if meta.RequestId == nil {
		return errorsNew("RequestId must not be nil")
	}
	if len(meta.RequestId) != cid.Length {
		return fmt.Errorf("invalid RequestId length: %v; expected length %v",
			len(meta.RequestId), cid.Length)
	}

	return rv.sigVerifier.Verify(encToken, pubKey, msg)
}
