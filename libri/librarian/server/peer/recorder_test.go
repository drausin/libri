package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQueryRecorder_Record_success(t *testing.T) {
	r := newQueryRecorder()

	r.Record(Response, Success)
	assert.Equal(t, uint64(1), r.responses.nQueries)
	assert.Equal(t, uint64(0), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.Equal(r.responses.earliest))

	// check that requests hasn't been touched
	assert.Equal(t, uint64(0), r.requests.nQueries)
	assert.Equal(t, uint64(0), r.requests.nErrors)
	assert.Equal(t, int64(0), r.requests.latest.Unix())
	assert.Equal(t, int64(0), r.requests.earliest.Unix())

	time.Sleep(1 * time.Second)

	r.Record(Response, Success)
	assert.Equal(t, uint64(2), r.responses.nQueries)
	assert.Equal(t, uint64(0), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.After(r.responses.earliest))

	r.Record(Request, Success)
	assert.Equal(t, uint64(1), r.requests.nQueries)
	assert.Equal(t, uint64(0), r.requests.nErrors)
	assert.True(t, r.requests.latest.Unix() > 0)
	assert.True(t, r.requests.earliest.Unix() > 0)
}

func TestQueryRecorder_Record_error(t *testing.T) {
	r := newQueryRecorder()

	r.Record(Response, Error)
	assert.Equal(t, uint64(1), r.responses.nQueries)
	assert.Equal(t, uint64(1), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.Equal(r.responses.earliest))

	// check that requests hasn't been touched
	assert.Equal(t, uint64(0), r.requests.nQueries)
	assert.Equal(t, uint64(0), r.requests.nErrors)
	assert.Equal(t, int64(0), r.requests.latest.Unix())
	assert.Equal(t, int64(0), r.requests.earliest.Unix())

	time.Sleep(1 * time.Second)

	r.Record(Response, Success)
	assert.Equal(t, uint64(2), r.responses.nQueries)
	assert.Equal(t, uint64(1), r.responses.nErrors)
	assert.True(t, r.responses.latest.Unix() > 0)
	assert.True(t, r.responses.earliest.Unix() > 0)
	assert.True(t, r.responses.latest.After(r.responses.earliest))

	r.Record(Request, Error)
	assert.Equal(t, uint64(1), r.requests.nQueries)
	assert.Equal(t, uint64(1), r.requests.nErrors)
	assert.True(t, r.requests.latest.Unix() > 0)
	assert.True(t, r.requests.earliest.Unix() > 0)
}

func TestQueryRecorder_Merge(t *testing.T) {
	r1:= newQueryRecorder()
	r1.Record(Response, Success)
	r1.Record(Response, Success)
	r1.Record(Response, Error)

	r2 := newQueryRecorder()
	r2.Record(Response, Success)
	r2.Record(Request, Success)
	r2.Record(Request, Success)
	r2.Record(Request, Error)
	assert.Equal(t, uint64(1), r2.responses.nQueries)
	assert.Equal(t, uint64(0), r2.responses.nErrors)
	assert.Equal(t, uint64(3), r2.requests.nQueries)
	assert.Equal(t, uint64(1), r2.requests.nErrors)
	assert.True(t, r1.responses.earliest.Before(r2.responses.earliest))
	assert.True(t, r1.responses.latest.Before(r2.responses.latest))

	r1.Merge(r2)

	assert.Equal(t, uint64(4), r1.responses.nQueries)
	assert.Equal(t, uint64(1), r1.responses.nErrors)
	assert.Equal(t, uint64(3), r1.requests.nQueries)
	assert.Equal(t, uint64(1), r1.requests.nErrors)

	// r1 still has earliest response time
	assert.True(t, r1.responses.earliest.Before(r2.responses.earliest))

	// r1 gets r2's latest response time
	assert.True(t, r1.responses.latest.Equal(r2.responses.latest))
}
