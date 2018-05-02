package goodwill

import "github.com/drausin/libri/libri/common/id"

type Doctor interface {
	// Healthy indicates whether the peer is currently deemed healthy.
	Healthy(peerID id.ID) bool
}

func NewDefaultDoctor() Doctor {
	return NewNaiveDoctor()
}

type naiveDoctor struct{}

func NewNaiveDoctor() Doctor {
	return &naiveDoctor{}
}

func (j *naiveDoctor) Healthy(peerID id.ID) bool {
	return true
}
