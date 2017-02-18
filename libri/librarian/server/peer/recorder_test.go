package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPeer_RecordSuccess(t *testing.T) {
	r := newQueryRecorder()

	r.Record(Response, Success)
	assert.Equal(t, uint64(1), r.responses.nQueries)
	assert.Equal(t, uint64(0), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.Equal(r.responses.earliest))
	time.Sleep(1 * time.Second)

	r.Record(Response, Success)
	assert.Equal(t, uint64(2), r.responses.nQueries)
	assert.Equal(t, uint64(0), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.After(r.responses.earliest))
}

func TestPeer_RecordError(t *testing.T) {
	r := newQueryRecorder()

	r.Record(Response, Error)
	assert.Equal(t, uint64(1), r.responses.nQueries)
	assert.Equal(t, uint64(1), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.Equal(r.responses.earliest))
	time.Sleep(1 * time.Second)

	r.Record(Response, Success)
	assert.Equal(t, uint64(2), r.responses.nQueries)
	assert.Equal(t, uint64(1), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.After(r.responses.earliest))
}
