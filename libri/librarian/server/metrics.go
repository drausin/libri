package server

import (
	"encoding/binary"
	"sync"

	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	envelopeLabel = "envelope"
	entryLabel    = "entry"
	pageLabel     = "page"
)

var (
	envelopeCountKey = []byte("envelope_stored_count")
	envelopeSizeKey  = []byte("envelope_stored_size")
	entryCountKey    = []byte("entry_stored_count")
	entrySizeKey     = []byte("entry_stored_size")
	pageCountKey     = []byte("page_stored_count")
	pageSizeKey      = []byte("page_stored_size")
)

type storageMetrics struct {
	count      *prom.CounterVec
	size       *prom.CounterVec
	serverSL   storage.StorerLoader
	serverSLMu *sync.Mutex
}

func newStorageMetrics(serverSL storage.StorerLoader) *storageMetrics {
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
	sm := &storageMetrics{
		count:      count,
		size:       size,
		serverSL:   serverSL,
		serverSLMu: new(sync.Mutex),
	}
	sm.initFromStorage()
	return sm
}

func (sm *storageMetrics) Add(doc *api.Document) error {
	bytes, err := proto.Marshal(doc)
	errors.MaybePanic(err) // should never happen
	switch doc.Contents.(type) {
	case *api.Document_Envelope:
		if err := sm.storedMetricAdd(envelopeCountKey, 1); err != nil {
			return err
		}
		if err := sm.storedMetricAdd(envelopeSizeKey, uint64(len(bytes))); err != nil {
			return err
		}
		sm.count.WithLabelValues(envelopeLabel).Inc()
		sm.size.WithLabelValues(envelopeLabel).Add(float64(len(bytes)))
	case *api.Document_Entry:
		if err := sm.storedMetricAdd(entryCountKey, 1); err != nil {
			return err
		}
		if err := sm.storedMetricAdd(entrySizeKey, uint64(len(bytes))); err != nil {
			return err
		}
		sm.count.WithLabelValues(entryLabel).Inc()
		sm.size.WithLabelValues(entryLabel).Add(float64(len(bytes)))
	case *api.Document_Page:
		if err := sm.storedMetricAdd(pageCountKey, 1); err != nil {
			return err
		}
		if err := sm.storedMetricAdd(pageSizeKey, uint64(len(bytes))); err != nil {
			return err
		}
		sm.count.WithLabelValues(pageLabel).Inc()
		sm.size.WithLabelValues(pageLabel).Add(float64(len(bytes)))
	}
	return nil
}

func (sm *storageMetrics) initFromStorage() {

	// envelope
	value, err := sm.getStored(envelopeCountKey)
	errors.MaybePanic(err)
	sm.count.WithLabelValues(envelopeLabel).Add(float64(value))
	value, err = sm.getStored(envelopeSizeKey)
	errors.MaybePanic(err)
	sm.size.WithLabelValues(envelopeLabel).Add(float64(value))

	// entry
	value, err = sm.getStored(entryCountKey)
	errors.MaybePanic(err)
	sm.count.WithLabelValues(entryLabel).Add(float64(value))
	value, err = sm.getStored(entrySizeKey)
	errors.MaybePanic(err)
	sm.size.WithLabelValues(entryLabel).Add(float64(value))

	// page
	value, err = sm.getStored(pageCountKey)
	errors.MaybePanic(err)
	sm.count.WithLabelValues(pageLabel).Add(float64(value))
	value, err = sm.getStored(pageSizeKey)
	errors.MaybePanic(err)
	sm.size.WithLabelValues(pageLabel).Add(float64(value))
}

func (sm *storageMetrics) storedMetricAdd(key []byte, amount uint64) error {
	sm.serverSLMu.Lock()
	defer sm.serverSLMu.Unlock()
	stored, err := sm.getStored(key)
	if err != nil {
		return err
	}
	stored += amount
	storedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(storedBytes, stored)
	return sm.serverSL.Store(key, storedBytes)
}

func (sm *storageMetrics) getStored(key []byte) (uint64, error) {
	var stored uint64
	storedBytes, err := sm.serverSL.Load(key)
	if err != nil {
		return stored, err
	}
	if storedBytes != nil {
		stored = binary.BigEndian.Uint64(storedBytes)
	}
	return stored, nil
}

func (sm *storageMetrics) register() {
	prom.MustRegister(sm.count)
	prom.MustRegister(sm.size)
	for _, docType := range []string{entryLabel, entryLabel, pageLabel} {
		_, err := sm.count.GetMetricWithLabelValues(docType)
		errors.MaybePanic(err) // should never happen
		_, err = sm.size.GetMetricWithLabelValues(docType)
		errors.MaybePanic(err) // should never happen
	}
}

func (sm *storageMetrics) unregister() {
	_ = prom.Unregister(sm.count)
	_ = prom.Unregister(sm.size)
}
