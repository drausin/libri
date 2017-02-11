package api

import (
	cid "github.com/drausin/libri/libri/common/id"
)

// NewFindRequest creates a FindRequest object.
func NewFindRequest(key cid.ID, nPeers uint) *FindRequest {
	return &FindRequest{
		RequestId: cid.NewRandom().Bytes(),
		Key:       key.Bytes(),
		NumPeers:  uint32(nPeers),
	}
}

// NewStoreRequest creates a StoreRequest object.
func NewStoreRequest(key cid.ID, value []byte) *StoreRequest {
	return &StoreRequest{
		RequestId: cid.NewRandom().Bytes(),
		Key:       key.Bytes(),
		Value:     value,
	}
}

// NewGetRequest creates a GetRequest object.
func NewGetRequest(key cid.ID) *GetRequest {
	return &GetRequest{
		RequestId: cid.NewRandom().Bytes(),
		Key: key.Bytes(),
	}
}

// NewPutRequest creates a PutRequest object.
func NewPutRequest(key cid.ID, value []byte) *PutRequest {
	return &PutRequest{
		RequestId: cid.NewRandom().Bytes(),
		Key: key.Bytes(),
		Value: value,
	}
}
