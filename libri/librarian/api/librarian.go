package api

import (
	cid "github.com/drausin/libri/libri/common/id"
)

// NewRequestMetadata creates a RequestMetadata object from the peer ID and a random request ID.
func NewRequestMatadata(peerID cid.ID) *RequestMetadata {
	return &RequestMetadata{
		RequestId: cid.NewRandom().Bytes(),
		PeerId: peerID.Bytes(),
	}
}

// NewFindRequest creates a FindRequest object.
func NewFindRequest(peerID cid.ID, key cid.ID, nPeers uint) *FindRequest {
	return &FindRequest{
		Metadata: NewRequestMatadata(peerID),
		Key:       key.Bytes(),
		NumPeers:  uint32(nPeers),
	}
}

// NewStoreRequest creates a StoreRequest object.
func NewStoreRequest(peerID cid.ID, key cid.ID, value []byte) *StoreRequest {
	return &StoreRequest{
		Metadata: NewRequestMatadata(peerID),
		Key:       key.Bytes(),
		Value:     value,
	}
}

// NewGetRequest creates a GetRequest object.
func NewGetRequest(peerID cid.ID, key cid.ID) *GetRequest {
	return &GetRequest{
		Metadata: NewRequestMatadata(peerID),
		Key:       key.Bytes(),
	}
}

// NewPutRequest creates a PutRequest object.
func NewPutRequest(peerID cid.ID, key cid.ID, value []byte) *PutRequest {
	return &PutRequest{
		Metadata: NewRequestMatadata(peerID),
		Key:       key.Bytes(),
		Value:     value,
	}
}
