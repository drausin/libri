package goodwill

import "gonum.org/v1/gonum/stat/distuv"

type BetaParameters struct {
	Alpha float64
	Beta  float64
}

func (p *BetaParameters) Dist() *distuv.Beta {
	return &distuv.Beta{
		Alpha: p.Alpha,
		Beta:  p.Beta,
	}
}

type BetaMetricParameters struct {
	IdealPrior *BetaParameters
	PeerPrior  *BetaParameters
}
