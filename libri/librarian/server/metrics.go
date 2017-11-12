package server

import (
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	envelopeLabel = "envelope"
	entryLabel    = "entry"
	pageLabel     = "page"
)

type storageMetrics struct {
	count *prom.CounterVec
	size  *prom.CounterVec
}

func newStorageMetrics() *storageMetrics {
	count := prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "grpc",
			Subsystem: "server",
			Name:      "doc_stored_count",
			Help:      "Total number of documents stored.",
		},
		[]string{"doc_type"},
	)
	size := prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: "grpc",
			Subsystem: "server",
			Name:      "doc_stored_size",
			Help:      "Total size (bytes) of documents stored.",
		},
		[]string{"doc_type"},
	)
	return &storageMetrics{
		count: count,
		size:  size,
	}
}

func (sm *storageMetrics) Add(doc *api.Document) {
	bytes, err := proto.Marshal(doc)
	errors.MaybePanic(err) // should never happen
	switch doc.Contents.(type) {
	case *api.Document_Envelope:
		sm.count.WithLabelValues(envelopeLabel).Inc()
		sm.size.WithLabelValues(envelopeLabel).Add(float64(len(bytes)))
	case *api.Document_Entry:
		sm.count.WithLabelValues(entryLabel).Inc()
		sm.size.WithLabelValues(entryLabel).Add(float64(len(bytes)))
	case *api.Document_Page:
		sm.count.WithLabelValues(pageLabel).Inc()
		sm.size.WithLabelValues(pageLabel).Add(float64(len(bytes)))
	}
}

func (sm *storageMetrics) register() {
	prom.MustRegister(sm.count)
	prom.MustRegister(sm.size)
	for _, docType := range []string{entryLabel, entryLabel, pageLabel} {
		sm.count.GetMetricWithLabelValues(docType)
		sm.size.GetMetricWithLabelValues(docType)
	}
}

func (sm *storageMetrics) unregister() {
	_ = prom.Unregister(sm.count)
	_ = prom.Unregister(sm.size)
}
