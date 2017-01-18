package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPeer_RecordResponseSuccess(t *testing.T) {
	rs := newResponseStats().(*responseStats)

	rs.Success()
	assert.Equal(t, uint64(1), rs.nQueries)
	assert.Equal(t, uint64(0), rs.nErrors)
	assert.True(t, rs.latest.Unix() > 0)
	assert.True(t, rs.earliest.Unix() > 0)
	assert.True(t, rs.latest.Equal(rs.earliest))
	time.Sleep(1 * time.Second)

	rs.Success()
	assert.Equal(t, uint64(2), rs.nQueries)
	assert.Equal(t, uint64(0), rs.nErrors)
	assert.True(t, rs.latest.Unix() > 0)
	assert.True(t, rs.earliest.Unix() > 0)
	assert.True(t, rs.latest.After(rs.earliest))
}

func TestPeer_RecordResponseError(t *testing.T) {
	rs := newResponseStats().(*responseStats)

	rs.Error()
	assert.Equal(t, uint64(1), rs.nQueries)
	assert.Equal(t, uint64(1), rs.nErrors)
	assert.True(t, rs.latest.Unix() > 0)
	assert.True(t, rs.earliest.Unix() > 0)
	assert.True(t, rs.latest.Equal(rs.earliest))

	rs.Success()
	assert.Equal(t, uint64(2), rs.nQueries)
	assert.Equal(t, uint64(1), rs.nErrors)
	assert.True(t, rs.latest.Unix() > 0)
	assert.True(t, rs.earliest.Unix() > 0)
	assert.True(t, rs.latest.After(rs.earliest))
}
