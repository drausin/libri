package client

import (
	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"errors"
)

// ErrUnexpectedRequestID indicates when the RequestID in a response is different than that in the
// request.
var ErrUnexpectedRequestID = errors.New("response contains unexpected RequestID")

// NewRequestMetadata creates a RequestMetadata object from the peer ID and a random request ID.
func NewRequestMetadata(peerID ecid.ID) *api.RequestMetadata {
	return &api.RequestMetadata{
		RequestId: cid.NewRandom().Bytes(),
		PubKey:    ecid.ToPublicKeyBytes(peerID),
	}
}

// NewIntroduceRequest creates an IntroduceRequest object.
func NewIntroduceRequest(
	peerID ecid.ID, apiSelf *api.PeerAddress, nPeers uint,
) *api.IntroduceRequest {
	return &api.IntroduceRequest{
		Metadata: NewRequestMetadata(peerID),
		Self:     apiSelf,
		NumPeers: uint32(nPeers),
	}
}

// NewFindRequest creates a FindRequest object.
func NewFindRequest(peerID ecid.ID, key cid.ID, nPeers uint) *api.FindRequest {
	return &api.FindRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
		NumPeers: uint32(nPeers),
	}
}

// NewStoreRequest creates a StoreRequest object.
func NewStoreRequest(peerID ecid.ID, key cid.ID, value *api.Document) *api.StoreRequest {
	return &api.StoreRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
		Value:    value,
	}
}

// NewGetRequest creates a GetRequest object.
func NewGetRequest(peerID ecid.ID, key cid.ID) *api.GetRequest {
	return &api.GetRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
	}
}

// NewPutRequest creates a PutRequest object.
func NewPutRequest(peerID ecid.ID, key cid.ID, value *api.Document) *api.PutRequest {
	return &api.PutRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
		Value:    value,
	}
}
