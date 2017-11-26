package server

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestStorageMetrics_Add(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	sm := newStorageMetrics()
	defer sm.unregister()
	envDoc := &api.Document{
		Contents: &api.Document_Envelope{
			Envelope: api.NewTestEnvelope(rng),
		},
	}
	sm.Add(envDoc)
	entryDoc := &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestSinglePageEntry(rng),
		},
	}
	sm.Add(entryDoc)
	pageDoc := &api.Document{
		Contents: &api.Document_Page{
			Page: api.NewTestPage(rng),
		},
	}
	sm.Add(pageDoc)

	// check we have a single count for each doc type
	countMetrics := make(chan prom.Metric, 3)
	expectedLabelValues := map[string]struct{}{
		"envelope": {},
		"entry":    {},
		"page":     {},
	}
	sm.count.Collect(countMetrics)
	close(countMetrics)
	nCountMetrics := 0
	actualCountLabelValues := map[string]struct{}{}
	for countMetric := range countMetrics {
		written := &dto.Metric{}
		countMetric.Write(written)
		assert.Equal(t, float64(1.0), *written.Counter.Value)
		assert.Equal(t, 1, len(written.Label))
		assert.Equal(t, "doc_type", *written.Label[0].Name)
		actualCountLabelValues[*written.Label[0].Value] = struct{}{}
		nCountMetrics++
	}
	assert.Equal(t, 3, nCountMetrics)
	assert.Equal(t, expectedLabelValues, actualCountLabelValues)

	sizeMetrics := make(chan prom.Metric, 3)
	sm.size.Collect(sizeMetrics)
	close(sizeMetrics)
	nSizeMetrics := 0
	actualSizeLabelValues := map[string]struct{}{}
	for sizeMetric := range sizeMetrics {
		written := &dto.Metric{}
		sizeMetric.Write(written)
		assert.True(t, *written.Counter.Value > float64(0.0))
		assert.Equal(t, 1, len(written.Label))
		assert.Equal(t, "doc_type", *written.Label[0].Name)
		actualSizeLabelValues[*written.Label[0].Value] = struct{}{}
		nSizeMetrics++
	}
	assert.Equal(t, 3, nSizeMetrics)
}
