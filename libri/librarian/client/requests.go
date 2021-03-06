package client

import (
	"errors"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// ErrUnexpectedRequestID indicates when the RequestID in a response is different than that in the
// request.
var ErrUnexpectedRequestID = errors.New("response contains unexpected RequestID")

// NewRequestMetadata creates a RequestMetadata object from the peer ID and a random request ID.
func NewRequestMetadata(peerID, orgID ecid.ID) *api.RequestMetadata {
	m := &api.RequestMetadata{
		RequestId: id.NewRandom().Bytes(),
		PubKey:    peerID.PublicKeyBytes(),
	}
	if orgID != nil {
		m.OrgPubKey = orgID.PublicKeyBytes()
	}
	return m
}

// NewIntroduceRequest creates an IntroduceRequest object.
func NewIntroduceRequest(
	peerID, orgID ecid.ID, apiSelf *api.PeerAddress, nPeers uint,
) *api.IntroduceRequest {
	return &api.IntroduceRequest{
		Metadata: NewRequestMetadata(peerID, orgID),
		Self:     apiSelf,
		NumPeers: uint32(nPeers),
	}
}

// NewFindRequest creates a FindRequest object.
func NewFindRequest(peerID, orgID ecid.ID, key id.ID, nPeers uint) *api.FindRequest {
	return &api.FindRequest{
		Metadata: NewRequestMetadata(peerID, orgID),
		Key:      key.Bytes(),
		NumPeers: uint32(nPeers),
	}
}

// NewVerifyRequest creates a VerifyRequest object.
func NewVerifyRequest(
	peerID, orgID ecid.ID, key id.ID, macKey []byte, nPeers uint,
) *api.VerifyRequest {
	return &api.VerifyRequest{
		Metadata: NewRequestMetadata(peerID, orgID),
		Key:      key.Bytes(),
		MacKey:   macKey,
		NumPeers: uint32(nPeers),
	}
}

// NewStoreRequest creates a StoreRequest object.
func NewStoreRequest(peerID, orgID ecid.ID, key id.ID, value *api.Document) *api.StoreRequest {
	return &api.StoreRequest{
		Metadata: NewRequestMetadata(peerID, orgID),
		Key:      key.Bytes(),
		Value:    value,
	}
}

// NewGetRequest creates a GetRequest object.
func NewGetRequest(peerID, orgID ecid.ID, key id.ID) *api.GetRequest {
	return &api.GetRequest{
		Metadata: NewRequestMetadata(peerID, orgID),
		Key:      key.Bytes(),
	}
}

// NewPutRequest creates a PutRequest object.
func NewPutRequest(peerID, orgID ecid.ID, key id.ID, value *api.Document) *api.PutRequest {
	return &api.PutRequest{
		Metadata: NewRequestMetadata(peerID, orgID),
		Key:      key.Bytes(),
		Value:    value,
	}
}

// NewSubscribeRequest creates a SubscribeRequest object.
func NewSubscribeRequest(
	peerID, orgID ecid.ID, subscription *api.Subscription,
) *api.SubscribeRequest {
	return &api.SubscribeRequest{
		Metadata:     NewRequestMetadata(peerID, orgID),
		Subscription: subscription,
	}
}
