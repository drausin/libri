package comm

import "github.com/drausin/libri/libri/common/id"

// Knower defines which peers are known, and thus usually more trustworthy, versus unknown.
type Knower interface {

	// Know returns whether a peer is known.
	Know(peerID id.ID) bool
}

// NewAlwaysKnower returns a Knower than treats all peers as known.
func NewAlwaysKnower() Knower {
	return &alwaysKnower{}
}

type alwaysKnower struct{}

func (k *alwaysKnower) Know(peerID id.ID) bool {
	return true
}
