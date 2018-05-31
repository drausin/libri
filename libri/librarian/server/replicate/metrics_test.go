package replicate

import (
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_incVerification(t *testing.T) {
	m := newMetrics()
	m.register()
	defer m.unregister()

	// do some incs
	for _, r := range []result{succeeded, exhausted, errored} {
		for _, s := range []replicationStatus{full, under} {
			m.incVerification(r, s)
		}
	}

	// check that we have a single count for each verification result and status
	verMetrics := make(chan prom.Metric, 6)
	m.verification.Collect(verMetrics)
	close(verMetrics)
	c := 0
	for verMetric := range verMetrics {
		written := dto.Metric{}
		verMetric.Write(&written)
		assert.Equal(t, float64(1.0), *written.Counter.Value)
		assert.Equal(t, 2, len(written.Label))
		c++
	}
	assert.Equal(t, 6, c)
}

func TestMetrics_incReplication(t *testing.T) {
	m := newMetrics()
	m.register()
	defer m.unregister()

	// do some incs
	for _, r := range []result{succeeded, exhausted, errored} {
		m.incReplication(r)
	}

	// check we have a single count for each replication result
	repMetrics := make(chan prom.Metric, 3)
	m.replication.Collect(repMetrics)
	close(repMetrics)
	c := 0
	for repMetric := range repMetrics {
		written := dto.Metric{}
		repMetric.Write(&written)
		assert.Equal(t, float64(1.0), *written.Counter.Value)
		assert.Equal(t, 1, len(written.Label))
		c++
	}
	assert.Equal(t, 3, c)
}
