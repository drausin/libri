package server

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/common/id"
	"fmt"
	"github.com/pkg/errors"
)

// RequestVerifier verifies requests by checking the signature in the context.
type RequestVerifier interface {
	Verify(ctx context.Context, msg proto.Message, meta *api.RequestMetadata) error
}

type requestVerifier struct {
	sigVerifier signature.Verifier
}

func NewRequestVerifier() RequestVerifier {
	return &requestVerifier{
		sigVerifier: signature.NewVerifier(),
	}
}

func (rv *requestVerifier) Verify(ctx context.Context, msg proto.Message,
	meta *api.RequestMetadata) error {
	encToken := ctx.Value(signature.ContextKey).(string)
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

	return rv.sigVerifier.Verify(encToken, pubKey, msg)
}
