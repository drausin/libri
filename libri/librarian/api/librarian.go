package api

import (
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/ecid"
)

// NewRequestMetadata creates a RequestMetadata object from the peer ID and a random request ID.
func NewRequestMetadata(peerID ecid.ID) *RequestMetadata {
	return &RequestMetadata{
		RequestId: cid.NewRandom().Bytes(),
		PubKey:    ecid.ToPublicKeyBytes(peerID),
	}
}

// NewFindRequest creates a FindRequest object.
func NewFindRequest(peerID ecid.ID, key cid.ID, nPeers uint) *FindRequest {
	return &FindRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
		NumPeers: uint32(nPeers),
	}
}

// NewStoreRequest creates a StoreRequest object.
func NewStoreRequest(peerID ecid.ID, key cid.ID, value []byte) *StoreRequest {
	return &StoreRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
		Value:    value,
	}
}

// NewGetRequest creates a GetRequest object.
func NewGetRequest(peerID ecid.ID, key cid.ID) *GetRequest {
	return &GetRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
	}
}

// NewPutRequest creates a PutRequest object.
func NewPutRequest(peerID ecid.ID, key cid.ID, value []byte) *PutRequest {
	return &PutRequest{
		Metadata: NewRequestMetadata(peerID),
		Key:      key.Bytes(),
		Value:    value,
	}
}
