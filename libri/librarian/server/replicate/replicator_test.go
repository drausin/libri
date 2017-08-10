package replicate

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultParameters(t *testing.T) {
	p := NewDefaultParameters()
	assert.NotZero(t, p.VerifyInterval)
	assert.NotZero(t, p.ReplicateConcurrency)
	assert.NotZero(t, p.MaxErrRate)
}

func TestMetrics_copy(t *testing.T) {
	m1 := &Metrics{
		NUnderreplicated: 1,
		NReplicated: 2,
		LatestPass: time.Now().Unix(),
	}
	m2 := m1.clone()
	assert.Equal(t, m1, m2)
}



