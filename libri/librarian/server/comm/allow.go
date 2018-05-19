package comm

import (
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrKnownAboveQueryLimit indicates when a known peer is above a query limit.
	ErrKnownAboveQueryLimit = errors.New("known peer above query limit")

	// ErrUnknownAboveQueryLimit indicates when an unknown peer is above a query limit.
	ErrUnknownAboveQueryLimit = errors.New("unknown peer above query limit")

	// ErrKnownAbovePeerLimit indicates when a known peer is above a peer limit.
	ErrKnownAbovePeerLimit = errors.New("known peer above peer limit")

	// ErrUnknownAbovePeerLimit indicates when an unknown peer is above a peer limit.
	ErrUnknownAbovePeerLimit = errors.New("unknown peer above peer limit")

	// ErrUnauthorized indicates when a peer is not authorized to make requests against an
	// endpoint.
	ErrUnauthorized = errors.New("unauthorized")

	defaultAuthorizations = Authorizations{
		api.Put:       {false: false, true: true},
		api.Get:       {false: false, true: true},
		api.Introduce: {false: false, true: true},
		api.Find:      {false: false, true: true},
		api.Verify:    {false: false, true: true},
		api.Store:     {false: false, true: true},
		api.Subscribe: {false: false, true: true},
	}

	// Second defines a second time window for a Recorder.
	Second = time.Second

	// Day defines a day time window for a Recorder.
	Day = 24 * time.Hour

	// Week defines a week time window for a Recorder.
	Week = 7 * Day

	defaultQueryWindowLimits = windowLimits{
		Second: Limits{
			api.Put:       {false: 0, true: 16},
			api.Get:       {false: 0, true: 16},
			api.Introduce: {false: 0, true: 16},
			api.Find:      {false: 0, true: 16},
			api.Verify:    {false: 0, true: 16},
			api.Store:     {false: 0, true: 16},
			api.Subscribe: {false: 0, true: 16},
		},
		Day: Limits{
			api.Put:       {false: 0, true: 256 * 1024},
			api.Get:       {false: 0, true: 256 * 1024},
			api.Introduce: {false: 0, true: 256 * 1024},
			api.Find:      {false: 0, true: 256 * 1024},
			api.Verify:    {false: 0, true: 256 * 1024},
			api.Store:     {false: 0, true: 256 * 1024},
			api.Subscribe: {false: 0, true: 256 * 1024},
		},
	}

	defaultPeerWindowLimits = windowLimits{
		Second: Limits{
			api.Put:       {false: 0, true: 64},
			api.Get:       {false: 0, true: 64},
			api.Introduce: {false: 0, true: 64},
			api.Find:      {false: 0, true: 64},
			api.Verify:    {false: 0, true: 64},
			api.Store:     {false: 0, true: 64},
			api.Subscribe: {false: 0, true: 64},
		},
		Day: Limits{
			api.Put:       {false: 0, true: 256},
			api.Get:       {false: 0, true: 256},
			api.Introduce: {false: 0, true: 256},
			api.Find:      {false: 0, true: 256},
			api.Verify:    {false: 0, true: 256},
			api.Store:     {false: 0, true: 256},
			api.Subscribe: {false: 0, true: 256},
		},
	}
)

// Allower decides whether peers should be allowed to make requests.
type Allower interface {

	// Allow determines whether a peer should be allowed to make a request on a given
	// endpoint. It returns an error with a grpc status code if the peer is not allowed.
	Allow(peerID id.ID, endpoint api.Endpoint) error
}

// NewDefaultAllower returns a new Allower using a
func NewDefaultAllower(knower Knower, queryGetters WindowQueryGetters) Allower {
	limits := make([]Limiter, 0)
	for window, peerWindowLimit := range defaultPeerWindowLimits {
		lim := NewPeerLimiter(peerWindowLimit, knower, queryGetters[window])
		limits = append(limits, lim)
	}
	for window, queryWindowLimit := range defaultQueryWindowLimits {
		lim := NewQueryLimiter(queryWindowLimit, knower, queryGetters[window])
		limits = append(limits, lim)
	}
	auth := NewConfiguredAuthorizer(defaultAuthorizations, knower)
	return NewAllower(auth, limits...)
}

// NewAllower returns a new Allower using the given Authorizer and Limiters.
func NewAllower(auth Authorizer, limiters ...Limiter) Allower {
	return &allower{
		auth:     auth,
		limiters: limiters,
	}
}

type allower struct {
	auth     Authorizer
	limiters []Limiter
}

func (a *allower) Allow(peerID id.ID, endpoint api.Endpoint) error {
	if err := a.auth.Authorized(peerID, endpoint); err != nil {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	for _, limiter := range a.limiters {
		if err := limiter.WithinLimit(peerID, endpoint); err != nil {
			return status.Error(codes.ResourceExhausted, err.Error())
		}
	}
	return nil
}

// Authorizer authorizes peers on endpoints.
type Authorizer interface {

	// Authorized determines whether a peer is authorized to make requests on a given endpoint.
	// It returns an error if the peer is not authorized.
	Authorized(peerID id.ID, endpoint api.Endpoint) error
}

// Authorizations defines whether a peer is authorized on an endpoint based on whether it is known
// or not.
type Authorizations map[api.Endpoint]map[bool]bool

// NewAlwaysAuthorizer returns an Authorizer that always
func NewAlwaysAuthorizer() Authorizer {
	return &alwaysAuthorizer{}
}

type alwaysAuthorizer struct{}

func (a *alwaysAuthorizer) Authorized(peerID id.ID, endpoint api.Endpoint) error {
	return nil
}

// NewConfiguredAuthorizer creates a new Authorizer from the given authorizations and using the
// given Knower.
func NewConfiguredAuthorizer(auths Authorizations, knower Knower) Authorizer {
	return &configuredAuthorizer{
		auths:  auths,
		knower: knower,
	}
}

type configuredAuthorizer struct {
	auths  Authorizations
	knower Knower
}

func (a *configuredAuthorizer) Authorized(peerID id.ID, endpoint api.Endpoint) error {
	known := a.knower.Know(peerID)
	if epAuth, hasEPAuth := a.auths[endpoint]; hasEPAuth {
		if knownAuth, hasKnownAuth := epAuth[known]; hasKnownAuth {
			if !knownAuth {
				return ErrUnauthorized
			}
		}
	}
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

type windowLimits map[time.Duration]Limits

// NewPeerLimiter returns a new Limiter on the number of peers allowed to make requests for certain
// endpoints and known status.
func NewPeerLimiter(limit Limits, knower Knower, qGetter QueryGetter) Limiter {
	return &peerLimiter{
		limit:   limit,
		knower:  knower,
		qGetter: qGetter,
	}
}

type peerLimiter struct {
	limit   Limits
	knower  Knower
	qGetter QueryGetter
}

func (l *peerLimiter) WithinLimit(peerID id.ID, endpoint api.Endpoint) error {
	if epLimit, hasEPLimit := l.limit[endpoint]; hasEPLimit {
		known := l.knower.Know(peerID)
		if knownLimit, hasKnownLimit := epLimit[known]; hasKnownLimit {
			nPeers := uint64(l.qGetter.CountPeers(endpoint, Request, known))
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

// NewQueryLimiter returns a new Limiter on the number of requests a peer can make for a given
// endpoint.
func NewQueryLimiter(limit Limits, knower Knower, qGetter QueryGetter) Limiter {
	return &queryLimiter{
		limit:   limit,
		knower:  knower,
		qGetter: qGetter,
	}
}

type queryLimiter struct {
	limit   Limits
	knower  Knower
	qGetter QueryGetter
}

func (l *queryLimiter) WithinLimit(peerID id.ID, endpoint api.Endpoint) error {
	if epLimit, hasEPLimit := l.limit[endpoint]; hasEPLimit {
		known := l.knower.Know(peerID)
		if knownLimit, hasKnownLimit := epLimit[known]; hasKnownLimit {
			qo := l.qGetter.Get(peerID, endpoint)[Request]
			nQueries := qo[Success].Count + qo[Error].Count
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
