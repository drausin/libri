package goodwill

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

type Truster interface {
	// Trust indicates whether a peer should be trusted with the given query type to the given
	// endpoint.
	Trust(peerID id.ID, endpoint api.Endpoint, queryType QueryType) bool
}

func NewDefaultTruster() Truster {
	return NewNaiveTruster()
}

type naiveTruster struct{}

func NewNaiveTruster() Truster {
	return &naiveTruster{}
}

func (j *naiveTruster) Trust(peerID id.ID, endpoint api.Endpoint, queryType QueryType) bool {
	return true
}
