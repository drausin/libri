package goodwill

import (
	"time"

	"sync"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"gonum.org/v1/gonum/stat/distuv"
)

var (
	defaultFindRqRpParams = &BetaMetricParameters{
		IdealPrior: &BetaParameters{
			Alpha: float64(10.0),
			Beta:  float64(10.0),
		},
		PeerPrior: &BetaParameters{
			Alpha: float64(1.1),
			Beta:  float64(1.1),
		},
	}

	defaultCacheTTL = 5 * time.Second
)

type Preferer interface {
	// Prefer indicates whether peer 1 should be preferred over peer 2 when when prioritization
	// is necessary.
	Prefer(peerID1, peerID2 id.ID) bool
}

func NewDefaultPrefer(rec Recorder) Preferer {
	return NewFindRqRpBetaPrefer(
		rec,
		defaultFindRqRpParams.IdealPrior.Dist(), // no observations, so posterior = prior
		defaultFindRqRpParams.PeerPrior,
		defaultCacheTTL,
	)
}

type findRqRpBetaPreferer struct {
	rec                      Recorder
	idealPosterior           distuv.LogProber
	peerPrior                *distuv.Beta
	cachedPeerPosteriorMeans map[string]*cachedValue
	cacheTTL                 time.Duration
}

func NewFindRqRpBetaPrefer(
	rec Recorder,
	idealPosterior distuv.LogProber,
	peerPrior *BetaParameters,
	cacheTTL time.Duration,
) Preferer {
	return &findRqRpBetaPreferer{
		rec:            rec,
		idealPosterior: idealPosterior,
		peerPrior:      peerPrior.Dist(),
		cacheTTL:       cacheTTL,
	}
}

func (j *findRqRpBetaPreferer) Prefer(peerID1, peerID2 id.ID) bool {
	logPDF1 := j.idealPosteriorLogProb(peerID1)
	logPDF2 := j.idealPosteriorLogProb(peerID2)
	return logPDF1 > logPDF2
}

func (j *findRqRpBetaPreferer) idealPosteriorLogProb(peerID id.ID) float64 {
	value, in := j.cachedPeerPosteriorMeans[peerID.String()]
	if !in {
		value = &cachedValue{
			calc: func() float64 {
				return j.idealPosterior.LogProb(j.getPeerPosteriorMean(peerID))
			},
			ttl: j.cacheTTL,
		}
		j.cachedPeerPosteriorMeans[peerID.String()] = value
	}
	return value.Get()
}

func (j *findRqRpBetaPreferer) getPeerPosteriorMean(peerID id.ID) float64 {
	nRqs := j.rec.Get(peerID, api.Find)[Request][Success].Count
	nRps := j.rec.Get(peerID, api.Find)[Response][Success].Count
	return distuv.Beta{
		// construct peer posterior from prior + observations
		Alpha: j.peerPrior.Alpha + float64(nRqs),
		Beta:  j.peerPrior.Beta + float64(nRps),
	}.Mean()
}

type cachedValue struct {
	value    float64
	calc     func() float64
	lastCalc time.Time
	ttl      time.Duration
	mu       sync.Mutex
}

func (cv *cachedValue) Get() float64 {
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
