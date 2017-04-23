package client

import (
	"errors"

	"github.com/golang/protobuf/proto"
)

// TestNoOpSigner implements the signature.Signer interface but just returns a dummy token.
type TestNoOpSigner struct{}

// Sign returns a dummy token.
func (s *TestNoOpSigner) Sign(m proto.Message) (string, error) {
	return "noop.token.sig", nil
}

// TestErrSigner implements the signature.Signer interface but always returns an error.
type TestErrSigner struct{}

// Sign returns an error.
func (s *TestErrSigner) Sign(m proto.Message) (string, error) {
	return "", errorsNew("some sign error")
}
