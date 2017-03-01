package client

import (
	"time"

	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
)

// NewSignedTimeoutContext creates a new context with a timeout and request signature.
func NewSignedTimeoutContext(signer signature.Signer, request proto.Message,
	timeout time.Duration) (context.Context, context.CancelFunc, error) {
	ctx := context.Background()

	// sign the message
	signedJWT, err := signer.Sign(request)
	if err != nil {
		return nil, func() {}, err
	}
	ctx = context.WithValue(ctx, signature.NewContextKey(), signedJWT)

	// add timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)

	return ctx, cancel, nil
}
