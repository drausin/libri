package api

import (
	"net"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestConnector_Connect_ok(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 11000}
	conn := NewConnector(addr)
	conn.(*connector).dialer = &fixedDialer{clientConn: &grpc.ClientConn{}}
	assert.Nil(t, conn.(*connector).client)

	lc1, err := conn.Connect()
	assert.Nil(t, err)
	assert.NotNil(t, lc1)

	// should return existing LC
	lc2, err := conn.Connect()
	assert.Nil(t, err)
	assert.NotNil(t, lc2)
	assert.Equal(t, lc1, lc2)
}

func TestConnector_Connect_err(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 11000}
	conn := NewConnector(addr)
	conn.(*connector).dialer = &fixedDialer{dialErr: errors.New("some Dial error")}

	lc, err := conn.Connect()
	assert.NotNil(t, err)
	assert.Nil(t, lc)
}

// hard to test Disconnect() b/c can't mock grpc.ClientConn

func TestConnector_Address(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 11000}
	conn := NewConnector(addr)
	assert.Equal(t, conn.Address(), addr)
}

type fixedDialer struct {
	clientConn *grpc.ClientConn
	dialErr    error
}

func (f *fixedDialer) Dial(addr *net.TCPAddr) (*grpc.ClientConn, error) {
	return f.clientConn, f.dialErr
}
