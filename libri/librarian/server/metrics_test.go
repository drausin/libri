package server

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestStorageMetrics_initAdd(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	serverSL := &fixedSL{stored: make(map[string][]byte)}

	sm1 := newStorageMetrics(serverSL)
	sm1.register()
	defer sm1.unregister()
	envDoc := &api.Document{
		Contents: &api.Document_Envelope{
			Envelope: api.NewTestEnvelope(rng),
		},
	}
	entryDoc := &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestSinglePageEntry(rng),
		},
	}
	pageDoc := &api.Document{
		Contents: &api.Document_Page{
			Page: api.NewTestPage(rng),
		},
	}
	for _, doc := range []*api.Document{envDoc, entryDoc, pageDoc} {
		err := sm1.Add(doc)
		assert.Nil(t, err)
	}

	// simulate server restarting and re-loading storage metrics
	sm2 := newStorageMetrics(serverSL)

	// check we have a single count for each doc type
	countMetrics := make(chan prom.Metric, 3)
	expectedLabelValues := map[string]struct{}{
		"envelope": {},
		"entry":    {},
		"page":     {},
	}
	sm2.count.Collect(countMetrics)
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
	sm2.size.Collect(sizeMetrics)
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

type fixedSL struct {
	stored map[string][]byte
}

func (f *fixedSL) Iterate(
	keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte),
) error {
	panic("implement me")
}

func (f *fixedSL) Load(key []byte) ([]byte, error) {
	if value, in := f.stored[string(key)]; in {
		return value, nil
	}
	return nil, nil
}

func (f *fixedSL) Store(key []byte, value []byte) error {
	f.stored[string(key)] = value
	return nil
}
