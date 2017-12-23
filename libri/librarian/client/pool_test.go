package client

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestNewDefaultLRUPool(t *testing.T) {
	p, err := NewDefaultLRUPool()
	assert.Nil(t, err)
	assert.NotNil(t, p)
}

func TestLRUPool_Get_ok(t *testing.T) {
	cc := &grpc.ClientConn{}
	dialer := &fixedDialer{conn: cc}
	closer := &fixedCloser{}
	p, err := newLRUPool(1, dialer, closer)
	assert.Nil(t, err)
	addr1, addr2 := "some address", "some other address"

	// creates new connection
	lc1, err := p.Get(addr1)
	assert.Nil(t, err)
	assert.NotNil(t, lc1)
	assert.Equal(t, 1, dialer.nCalls)

	// uses same connection
	lc2, err := p.Get(addr1)
	assert.Nil(t, err)
	assert.NotNil(t, lc2)
	assert.Equal(t, lc1, lc2)
	assert.Equal(t, 1, dialer.nCalls)

	// creates new connection, evicting old connection
	lc3, err := p.Get(addr2)
	assert.Nil(t, err)
	assert.NotNil(t, lc3)
	assert.Equal(t, 2, dialer.nCalls)
}

func TestLRUPool_Get_err(t *testing.T) {
	p, err := newLRUPool(
		1,
		&fixedDialer{err: errors.New("some dial error")},
		&fixedCloser{err: errors.New("some close error")},
	)
	assert.Nil(t, err)
	lc, err := p.Get("some address")
	assert.NotNil(t, err)
	assert.Nil(t, lc)

	p.(*lruPool).dialer = &fixedDialer{conn: &grpc.ClientConn{}}
	lc, err = p.Get("some address")
	assert.Nil(t, err)
	assert.NotNil(t, lc)

	lc, err = p.Get("some other address")
	assert.NotNil(t, err)
	assert.Nil(t, lc)
}

func TestLRUPool_CloseAll_ok(t *testing.T) {
	cc := &grpc.ClientConn{}
	dialer := &fixedDialer{conn: cc}
	closer := &fixedCloser{}
	p, err := newLRUPool(2, dialer, closer)
	assert.Nil(t, err)
	addr1, addr2 := "some address", "some other address"

	lc1, err := p.Get(addr1)
	assert.Nil(t, err)
	assert.NotNil(t, lc1)
	lc2, err := p.Get(addr2)
	assert.Nil(t, err)
	assert.NotNil(t, lc2)

	err = p.CloseAll()
	assert.Nil(t, err)
	assert.Equal(t, 0, p.(*lruPool).conns.Len())
}

func TestLRUPool_CloseAll_err(t *testing.T) {
	cc := &grpc.ClientConn{}
	dialer := &fixedDialer{conn: cc}
	closer := &fixedCloser{err: errors.New("some close error")}
	p, err := newLRUPool(2, dialer, closer)
	assert.Nil(t, err)
	addr1, addr2 := "some address", "some other address"

	lc1, err := p.Get(addr1)
	assert.Nil(t, err)
	assert.NotNil(t, lc1)
	lc2, err := p.Get(addr2)
	assert.Nil(t, err)
	assert.NotNil(t, lc2)

	err = p.CloseAll()
	assert.NotNil(t, err)
}

type fixedDialer struct {
	conn   *grpc.ClientConn
	err    error
	nCalls int
}

func (fd *fixedDialer) dial(address string) (*grpc.ClientConn, error) {
	fd.nCalls++
	return fd.conn, fd.err
}

type fixedCloser struct {
	err error
}

func (fc *fixedCloser) close(conn *grpc.ClientConn) error {
	return fc.err
}
