package replicate

import (
	"github.com/drausin/libri/libri/common/errors"
	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	promNamespace = "libri"
	promSubsystem = "server_replicator"
)

type result int

const (
	succeeded result = iota
	exhausted
	errored
)

func (r result) String() string {
	switch r {
	case succeeded:
		return "succeeded"
	case exhausted:
		return "exhausted"
	case errored:
		return "errored"
	}
	panic("should never get here")
}

type replicationStatus int

const (
	unknown replicationStatus = iota
	full
	under
)

func (s replicationStatus) String() string {
	switch s {
	case unknown:
		return "unknown"
	case full:
		return "full"
	case under:
		return "under"
	}
	panic("should never get here")
}

type metrics struct {
	verification *prom.CounterVec
	replication  *prom.CounterVec
}

func newMetrics() *metrics {
	verifications := prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "verification_count",
			Help:      "Verifications event results and status counts",
		},
		[]string{"result", "status"},
	)
	replications := prom.NewCounterVec(
		prom.CounterOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "replication_count",
			Help:      "Replication event result counts",
		},
		[]string{"result"},
	)
	return &metrics{
		verification: verifications,
		replication:  replications,
	}
}

func (m *metrics) incVerification(result result, status replicationStatus) {
	m.verification.WithLabelValues(result.String(), status.String()).Inc()
}

func (m *metrics) incReplication(result result) {
	m.replication.WithLabelValues(result.String()).Inc()
}

func (m *metrics) register() {
	prom.MustRegister(m.verification)
	prom.MustRegister(m.replication)

	// populate zero counts
	for _, r := range []result{succeeded, exhausted, errored} {
		for _, s := range []replicationStatus{full, under} {
			_, err := m.verification.GetMetricWithLabelValues(r.String(), s.String())
			errors.MaybePanic(err) // should never happen
		}
		_, err := m.replication.GetMetricWithLabelValues(r.String())
		errors.MaybePanic(err) // should never happen
	}
}

func (m *metrics) unregister() {
	prom.Unregister(m.verification)
	prom.Unregister(m.replication)
}
