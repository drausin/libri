package comm

import "github.com/drausin/libri/libri/common/id"

type Knower interface {
	Know(peerID id.ID) bool
}

func NewNeverKnower() Knower {
	return &neverKnower{}
}

type neverKnower struct{}

func (k *neverKnower) Know(peerID id.ID) bool {
	return false
}
