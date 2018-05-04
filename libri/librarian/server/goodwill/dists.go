package goodwill

import "gonum.org/v1/gonum/stat/distuv"

// BetaParameters contains the parameters for a beta distribution.
type BetaParameters struct {
	Alpha float64
	Beta  float64
}

// Dist returns a Beta distribution instance from the given parameters.
func (p *BetaParameters) Dist() *distuv.Beta {
	return &distuv.Beta{
		Alpha: p.Alpha,
		Beta:  p.Beta,
	}
}

// BetaMetricParameters contains params required for a beta-distributed metric around a peer.
type BetaMetricParameters struct {
	IdealPrior *BetaParameters
	PeerPrior  *BetaParameters
}
