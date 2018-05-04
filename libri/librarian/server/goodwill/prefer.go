package goodwill

import (
	"sync"
	"time"

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

// Preferer judges whether one peer is preferable over another.
type Preferer interface {

	// Prefer indicates whether peer 1 should be preferred over peer 2 when prioritization
	// is necessary.
	Prefer(peerID1, peerID2 id.ID) bool
}

// NewDefaultPreferer returns a new default Preferer.
func NewDefaultPreferer(rec Recorder) Preferer {
	return NewDefaultFindRqRpBetaPreferer(rec)
}

type findRqRpBetaPreferer struct {
	rec                      Recorder
	idealPosterior           distuv.LogProber
	peerPrior                *distuv.Beta
	cachedPeerPosteriorMeans map[string]*cachedValue
	cacheTTL                 time.Duration
}

// NewFindRqRpBetaPreferer returns a new Preferer for Find endpoint request/(request + response)
// ratio. It takes in the recorder to use when getting endpoint stats, the ideal distribution
// posterior, the prior distribution for a peer, and a time to live (TTL) for cached values of the
// peer posterior log prob under the ideal posterior distribution.
func NewFindRqRpBetaPreferer(
	rec Recorder,
	idealPosterior distuv.LogProber,
	peerPrior *BetaParameters,
	cacheTTL time.Duration,
) Preferer {
	return &findRqRpBetaPreferer{
		rec:                      rec,
		idealPosterior:           idealPosterior,
		peerPrior:                peerPrior.Dist(),
		cachedPeerPosteriorMeans: make(map[string]*cachedValue),
		cacheTTL:                 cacheTTL,
	}
}

// NewDefaultFindRqRpBetaPreferer returns a new Preferer using the default ideal and peer priors
// for the Find rq/(rq + rp) distribution.
func NewDefaultFindRqRpBetaPreferer(rec Recorder) Preferer {
	return NewFindRqRpBetaPreferer(
		rec,
		defaultFindRqRpParams.IdealPrior.Dist(), // no observations, so posterior = prior
		defaultFindRqRpParams.PeerPrior,
		defaultCacheTTL,
	)
}

func (j *findRqRpBetaPreferer) Prefer(peerID1, peerID2 id.ID) bool {
	lp1, lp2 := j.idealPosteriorLogProb(peerID1), j.idealPosteriorLogProb(peerID2)
	return lp1 > lp2
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
	return value.get()
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
