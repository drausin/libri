package comm

import (
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/id"
)

// Preferer judges whether one peer is preferable over another.
type Preferer interface {

	// Prefer indicates whether peer 1 should be preferred over peer 2 when prioritization
	// is necessary.
	Prefer(peerID1, peerID2 id.ID) bool
}

type cachedValue struct {
	value    float64
	calc     func() float64
	lastCalc time.Time
	ttl      time.Duration
	mu       sync.Mutex
}

func (cv *cachedValue) get() float64 {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	now := time.Now()
	if cv.lastCalc.Add(cv.ttl).Before(now) {
		// TTL expired, so need to recalc
		cv.value = cv.calc()
		cv.lastCalc = now
	}
	return cv.value
}
