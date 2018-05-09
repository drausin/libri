package comm

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

type Limit map[api.Endpoint]uint64

type Limits struct {
	knownQuery   Limit
	unknownQuery Limit
	knownPeer    Limit
	unknownPeer  Limit
}

type Limiter interface {
	WithinLimits(peerID id.ID, endpoint api.Endpoint) (bool, bool)
}

type limiter struct {
	limits Limits
	rec    WindowRecorder
	knower Knower
}

func (l *limiter) WithinLimit(peerID id.ID, endpoint api.Endpoint) (bool, bool) {
	known := l.knower.Know(peerID)
	nPeers := uint64(l.rec.Count(endpoint, Request))
	if known && nPeers > l.limits.knownPeer[endpoint] {
		// too many known peers for given endpoint
		return false, true
	}
	if !known && nPeers > l.limits.unknownPeer[endpoint] {
		// too many unknown peers for given endpoint
		return false, true
	}

	qo := l.rec.Get(peerID, endpoint)[Request]
	nQueries := qo[Success].Count + qo[Error].Count
	if known {
		return true, nQueries <= l.limits.knownQuery[endpoint]
	}
	return true, nQueries < l.limits.unknownQuery[endpoint]
}
