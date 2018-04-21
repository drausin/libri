package server

import (
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	gw "github.com/drausin/libri/libri/librarian/server/goodwill"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
)

// newStubPeerFromPublicKeyBytes creates a new stub peer with an ID coming from an ECDSA public key.
func newIDFromPublicKeyBytes(pubKeyBytes []byte) (id.ID, error) {
	pubKey, err := ecid.FromPublicKeyBytes(pubKeyBytes)
	if err != nil {
		return nil, err
	}
	return id.FromPublicKey(pubKey), nil
}

// NewResponseMetadata creates a new api.ResponseMatadata object with the same RequestID as that
// in the api.RequestMetadata.
func (l *Librarian) NewResponseMetadata(m *api.RequestMetadata) *api.ResponseMetadata {
	return &api.ResponseMetadata{
		RequestId: m.RequestId,
		PubKey:    l.selfID.PublicKeyBytes(),
	}
}

// checkRequest verifies the request signature and records an error with the peer if necessary. It
// returns the ID of the requester or an error.
func (l *Librarian) checkRequest(
	ctx context.Context, rq proto.Message, meta *api.RequestMetadata,
) (id.ID, error) {
	requesterID, err := newIDFromPublicKeyBytes(meta.PubKey)
	if err != nil {
		return nil, err
	}

	// record request verification issue, if it exists
	if err := l.rqv.Verify(ctx, rq, meta); err != nil {
		return requesterID, err
	}
	return requesterID, nil
}

// checkRequestAndKey verifies the request signature and key, recording errors with the peer if
// necessary. It returns the ID of the requester or an error.
func (l *Librarian) checkRequestAndKey(
	ctx context.Context,
	rq proto.Message,
	meta *api.RequestMetadata,
	key []byte,
) (id.ID, error) {
	requesterID, err := l.checkRequest(ctx, rq, meta)
	if err != nil {
		return nil, err
	}
	if err := l.kc.Check(key); err != nil {
		return nil, err
	}
	return requesterID, nil
}

// checkRequestAndKey verifies the request signature and key/value combo, recording errors with
// the peer if necessary. It returns the ID of the requester or an error.
func (l *Librarian) checkRequestAndKeyValue(
	ctx context.Context,
	rq proto.Message,
	meta *api.RequestMetadata,
	key []byte,
	value *api.Document,
) (id.ID, error) {
	requesterID, err := l.checkRequest(ctx, rq, meta)
	if err != nil {
		return nil, err
	}
	valueBytes, err := proto.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err := l.kvc.Check(key, valueBytes); err != nil {
		return nil, err
	}
	return requesterID, nil
}

// record records query outcome for a particular peer if that peer is in the
// routing table.
func (l *Librarian) record(fromPeerID id.ID, e api.Endpoint, qt gw.QueryType, o gw.Outcome) {
	if fromPeerID == nil {
		return
	}
	l.rec.Record(fromPeerID, e, qt, o)
	if fromPeer, exists := l.rt.Get(fromPeerID); exists {
		// re-heap; if this proves expensive, we could choose to only selectively re-heap
		// when it changes the outcome of fromPeer.Before()
		l.rt.Push(fromPeer)
	}
}

func logAndReturnErr(logger *zap.Logger, msg string, err error) error {
	logger.Error(msg, zap.Error(err))
	return err
}

func logFieldsAndReturnErr(logger *zap.Logger, err error, fields []zapcore.Field) error {
	logger.Error(err.Error(), fields...)
	return err
}
