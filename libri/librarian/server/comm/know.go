package comm

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

type Knower interface {
	Know(peerID id.ID) bool
}

type Authorizer interface {
	Authorized(peerID id.ID, endpoint api.Endpoint) bool
}
