package api

import (
	cid "github.com/drausin/libri/libri/common/id"
)

// NewFindRequest creates a FindRequest object.
func NewFindRequest(target cid.ID, nPeers uint) *FindRequest {
	return &FindRequest{
		RequestId: cid.NewRandom().Bytes(),
		Target: target.Bytes(),
		NumPeers: uint32(nPeers),
	}
}
