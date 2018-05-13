package comm

import "github.com/drausin/libri/libri/common/id"

// Knower defines which peers are known, and thus usually more trustworthy, versus unknown.
type Knower interface {

	// Know returns whether a peer is known.
	Know(peerID id.ID) bool
}

// NewNeverKnower returns a Knower than treats all peers as unknown.
func NewNeverKnower() Knower {
	return &neverKnower{}
}

type neverKnower struct{}

func (k *neverKnower) Know(peerID id.ID) bool {
	return false
}
