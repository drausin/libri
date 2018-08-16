package server

import (
	"errors"
	"fmt"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/golang/protobuf/proto"
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

func (rv *verifier) Verify(
	ctx context.Context, msg proto.Message, meta *api.RequestMetadata,
) error {
	encToken, encOrgToken, err := client.FromSignatureContext(ctx)
	if err != nil {
		return err
	}
	pubKey, err := ecid.FromPublicKeyBytes(meta.PubKey)
	if err != nil {
		return err
	}
	if meta.RequestId == nil {
		return errors.New("RequestId must not be nil")
	}
	if len(meta.RequestId) != id.Length {
		return fmt.Errorf("invalid RequestId length: %v; expected length %v",
			len(meta.RequestId), id.Length)
	}
	if err = rv.sigVerifier.Verify(encToken, pubKey, msg); err != nil || encOrgToken == "" {
		return err
	}
	orgPubKey, err := ecid.FromPublicKeyBytes(meta.OrgPubKey)
	if err != nil {
		return err
	}
	return rv.sigVerifier.Verify(encOrgToken, orgPubKey, msg)
}
