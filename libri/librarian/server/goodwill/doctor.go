package goodwill

import "github.com/drausin/libri/libri/common/id"

// Doctor determines whether a peer is currently healthy.
type Doctor interface {

	// Healthy indicates whether the peer is currently deemed healthy.
	Healthy(peerID id.ID) bool
}

// NewDefaultDoctor returns a default Doctor.
func NewDefaultDoctor() Doctor {
	return NewNaiveDoctor()
}

type naiveDoctor struct{}

// NewNaiveDoctor returns a Doctor that naively assumes all peers are healthy.
func NewNaiveDoctor() Doctor {
	return &naiveDoctor{}
}

func (j *naiveDoctor) Healthy(peerID id.ID) bool {
	return true
}
