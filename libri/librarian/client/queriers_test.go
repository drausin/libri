package client

import (
	"testing"

	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/stretchr/testify/assert"
)

// Explanation: ideally would have unit test like this, but mocking an api.LibrarianClient is
// annoying b/c there are so many service methods. Will have to rely on integration tests to cover
// this branch.
//
// func Test(Introduce|Find|Store)Querier_Query_ok(t *testing.T) {}

func TestIntroduceQuerier_Query_err(t *testing.T) {
	c := &peer.TestErrConnector{}
	q := NewIntroduceQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}

func TestFindQuerier_Query_err(t *testing.T) {
	c := &peer.TestErrConnector{}
	q := NewFindQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}

func TestStoreQuerier_Query_err(t *testing.T) {
	c := &peer.TestErrConnector{}
	q := NewStoreQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}

func TestGetQuerier_Query_err(t *testing.T) {
	c := &peer.TestErrConnector{}
	q := NewGetQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}

func TestPutQuerier_Query_err(t *testing.T) {
	c := &peer.TestErrConnector{}
	q := NewPutQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}
