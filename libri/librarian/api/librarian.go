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

func NewStoreRequest(key cid.ID, value []byte) *StoreRequest {
	return &StoreRequest{
		RequestId: cid.NewRandom().Bytes(),
		Key:       key.Bytes(),
		Value:     value,
	}
}
