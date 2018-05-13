package comm

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
)

var (
	ErrKnownAboveQueryLimit   = errors.New("known peer above query limit")
	ErrUnknownAboveQueryLimit = errors.New("unknown peer above query limit")
	ErrKnownAbovePeerLimit    = errors.New("known peer above peer limit")
	ErrUnknownAbovePeerLimit  = errors.New("unknown peer above peer limit")
)

type Allower interface {
	Allow(peerID id.ID, endpoint api.Endpoint) error
}

func NewAllower(auth Authorizer, peer, query Limiter) Allower {
	return &allower{
		auth:  auth,
		peer:  peer,
		query: query,
	}
}

type allower struct {
	auth  Authorizer
	peer  Limiter
	query Limiter
}

func (a *allower) Allow(peerID id.ID, endpoint api.Endpoint) error {
	if err := a.auth.Authorized(peerID, endpoint); err != nil {
		return err
	}
	if err := a.peer.WithinLimit(peerID, endpoint); err != nil {
		return err
	}
	if err := a.query.WithinLimit(peerID, endpoint); err != nil {
		return err
	}
	return nil
}

type Authorizer interface {
	Authorized(peerID id.ID, endpoint api.Endpoint) error
}

func NewAlwaysAuthorizer() Authorizer {
	return &alwaysAuthorizer{}
}

type alwaysAuthorizer struct{}

func (a *alwaysAuthorizer) Authorized(peerID id.ID, endpoint api.Endpoint) error {
	return nil
}

// Limiter determines whether requests from peers are with a set of rate limits.
type Limiter interface {

	// WithinLimit determines whether a given peer's request on a given endpoint is within the
	// configured limit. It returns nil if it is within the limit or an error if not.
	WithinLimit(peerID id.ID, endpoint api.Endpoint) error
}

// Limits defines a set of limits for endpoints and whether the peer is known or not.
type Limits map[api.Endpoint]map[bool]uint64

func NewPeerLimiter(limit Limits, knower Knower, rec WindowRecorder) Limiter {
	return &peerLimiter{
		limit:  limit,
		knower: knower,
		rec:    rec,
	}
}

type peerLimiter struct {
	limit  Limits
	knower Knower
	rec    WindowRecorder
}

func (l *peerLimiter) WithinLimit(peerID id.ID, endpoint api.Endpoint) error {
	known := l.knower.Know(peerID)
	nPeers := uint64(l.rec.CountPeers(endpoint, Request, known))
	if epLimit, hasEPLimit := l.limit[endpoint]; hasEPLimit {
		if knownLimit, hasKnownLimit := epLimit[known]; hasKnownLimit {
			if nPeers > knownLimit {
				if known {
					return ErrKnownAbovePeerLimit
				}
				return ErrUnknownAbovePeerLimit
			}
		}
	}
	return nil
}

func NewQueryLimiter(limit Limits, knower Knower, rec WindowRecorder) Limiter {
	return &queryLimiter{
		limit:  limit,
		knower: knower,
		rec:    rec,
	}
}

type queryLimiter struct {
	limit  Limits
	knower Knower
	rec    WindowRecorder
}

func (l *queryLimiter) WithinLimit(peerID id.ID, endpoint api.Endpoint) error {
	qo := l.rec.Get(peerID, endpoint)[Request]
	nQueries := qo[Success].Count + qo[Error].Count
	known := l.knower.Know(peerID)
	if epLimit, hasEPLimit := l.limit[endpoint]; hasEPLimit {
		if knownLimit, hasKnownLimit := epLimit[known]; hasKnownLimit {
			if nQueries > knownLimit {
				if known {
					return ErrKnownAboveQueryLimit
				}
				return ErrUnknownAboveQueryLimit
			}
		}
	}
	return nil
}
