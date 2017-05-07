package subscribe

import (
	"fmt"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestToFromAPI(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	f1 := newFilter([][]byte{}, 0.75, rng)
	a, err := ToAPI(f1)
	assert.Nil(t, err)
	f2, err := FromAPI(a)
	assert.Nil(t, err)
	assert.Equal(t, f1, f2)
}

func TestFromAPI_err(t *testing.T) {
	f, err := FromAPI(&api.BloomFilter{Encoded: []byte{}})
	assert.NotNil(t, err)
	assert.Nil(t, f)
}

/*
// this "test" is helpful in empirically determining reasonable n and fp params for bloom filters
func TestEmpiricalFilterParameters(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	fps := []float64{0.0001, 0.001, 0.01, 0.1, 0.3, 0.5, 0.75, 0.9}
	ns := []uint{1, 2, 5, 10, 25, 50, 100}
	for _, n := range ns {
		for _, fp := range fps {
			m, k := bloom.EstimateParameters(n, fp)
			filter := bloom.New(m, k)
			for c := uint(0); c < n; c++ {
				filter.Add(api.RandBytes(rng, api.ECPubKeyLength))
			}
			fpCount, nTrials := 0, 100000
			for c := 0; c < nTrials; c++ {
				if filter.Test(api.RandBytes(rng, api.ECPubKeyLength)) {
					fpCount++
				}
			}
			actualFp := float64(fpCount) / float64(nTrials)
			log.Printf("n: %d, est fp: %f, m: %d, k: %d, actual fp: %f\n", n, fp, m,
				k, actualFp)
		}
	}
}
*/

func TestNewFPFilter(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	nTrials := 100000

	// check that for the actual FP rate is >= target FP rate - tolerance
	tolerance := 0.05
	for _, targetFP := range []float64{0.3, 0.5, 0.75, 0.9, 1.0} {
		filter := newFilter([][]byte{}, targetFP, rng)

		// measure FP rate
		fpCount := 0
		for c := 0; c < nTrials; c++ {
			if filter.Test(api.RandBytes(rng, api.ECPubKeyLength)) {
				fpCount++
			}
		}
		measuredFP := float64(fpCount) / float64(nTrials)
		info := fmt.Sprintf("target fp: %f, measured fp: %f", targetFP, measuredFP)
		assert.True(t, measuredFP >= targetFP - tolerance, info)
	}

}

func TestAlwaysInFilter(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	f := alwaysInFilter()
	for c := 0; c < 10000; c++ {
		in := f.Test(api.RandBytes(rng, api.ECPubKeyLength))
		assert.True(t, in)
	}
}
