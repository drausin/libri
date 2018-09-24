package comm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryType_String(t *testing.T) {
	assert.Equal(t, "REQUEST", Request.String())
	assert.Equal(t, "RESPONSE", Response.String())
}

func TestOutcome_String(t *testing.T) {
	assert.Equal(t, "SUCCESS", Success.String())
	assert.Equal(t, "ERROR", Error.String())
}

func TestMetrics_Record(t *testing.T) {
	m := newScalarMetrics()

	m.Record()
	assert.NotEmpty(t, m.Earliest)
	assert.Equal(t, m.Earliest, m.Latest)
	assert.Equal(t, uint64(1), m.Count)

	m.Record()
	assert.True(t, m.Earliest.Before(m.Latest))
	assert.Equal(t, uint64(2), m.Count)
}
