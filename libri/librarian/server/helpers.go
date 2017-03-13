package server

import (
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
)

// newStubPeerFromPublicKeyBytes creates a new stub peer with an ID coming from an ECDSA public key.
func newIDFromPublicKeyBytes(pubKeyBytes []byte) (cid.ID, error) {
	pubKey, err := ecid.FromPublicKeyBytes(pubKeyBytes)
	if err != nil {
		return nil, err
	}
	return cid.FromPublicKey(pubKey), nil
}

// NewResponseMetadata creates a new api.ResponseMatadata object with the same RequestID as that
// in the api.RequestMetadata.
func (l *Librarian) NewResponseMetadata(m *api.RequestMetadata) *api.ResponseMetadata {
	return &api.ResponseMetadata{
		RequestId: m.RequestId,
		PubKey:    ecid.ToPublicKeyBytes(l.selfID),
	}
}

// checkRequest verifies the request signature and records an error with the peer if necessary. It
// returns the ID of the requester or an error.
func (l *Librarian) checkRequest(ctx context.Context, rq proto.Message, meta *api.RequestMetadata) (
	cid.ID, error) {
	requesterID, err := newIDFromPublicKeyBytes(meta.PubKey)
	if err != nil {
		return nil, err
	}

	// record request verification issue, if it exists
	if err := l.rqv.Verify(ctx, rq, meta); err != nil {
		l.record(requesterID, peer.Request, peer.Error)
		return nil, err
	}
	return requesterID, nil
}

// checkRequestAndKey verifies the request signature and key, recording errors with the peer if
// necessary. It returns the ID of the requester or an error.
func (l *Librarian) checkRequestAndKey(ctx context.Context, rq proto.Message,
	meta *api.RequestMetadata, key []byte) (cid.ID, error) {
	requester, err := l.checkRequest(ctx, rq, meta)
	if err != nil {
		return nil, err
	}
	if err := l.kc.Check(key); err != nil {
		l.record(requester, peer.Request, peer.Error)
		return nil, err
	}
	return requester, nil
}

// checkRequestAndKey verifies the request signature and key/value combo, recording errors with
// the peer if necessary. It returns the ID of the requester or an error.
func (l *Librarian) checkRequestAndKeyValue(ctx context.Context, rq proto.Message,
	meta *api.RequestMetadata, key []byte, value *api.Document) (cid.ID, error) {
	requester, err := l.checkRequest(ctx, rq, meta)
	if err != nil {
		return nil, err
	}
	valueBytes, err := proto.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := l.kvc.Check(key, valueBytes); err != nil {
		l.record(requester, peer.Request, peer.Error)
		return nil, err
	}
	return requester, nil
}

// record records query outcome for a particular peer if that peer is in the routing table.
func (l *Librarian) record(fromPeerID cid.ID, t peer.QueryType, o peer.Outcome) {
	if existing := l.rt.Get(fromPeerID); existing != nil {
		// only record query outcomes for peers already in our routing table
		existing.Recorder().Record(t, o)

		// re-heap; if this proves expensive, we could choose to only selectively re-heap
		// when it changes the outcome of peer.Before()
		l.rt.Push(existing)
	}
}
