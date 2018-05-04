package goodwill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBetaParameters_Dist(t *testing.T) {
	p := &BetaParameters{Alpha: 2, Beta: 2}
	d := p.Dist()
	assert.Equal(t, p.Alpha, d.Alpha)
	assert.Equal(t, p.Beta, d.Beta)
}
