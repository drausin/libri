package peer

import (
	"net"
	"testing"

	"errors"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestConnector_Connect_ok(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}
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
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}
	conn := NewConnector(addr)
	conn.(*connector).dialer = &fixedDialer{dialErr: errors.New("some Dial error")}

	lc, err := conn.Connect()
	assert.NotNil(t, err)
	assert.Nil(t, lc)
}

// hard to test Disconnect() b/c can't mock grpc.ClientConn

func TestConnector_Address(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}
	conn := NewConnector(addr)
	assert.Equal(t, conn.Address(), addr)
}

func TestConnector_merge_diffAddress(t *testing.T) {
	a1 := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}
	a2 := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20101}
	c1 := &connector{publicAddress: a1}
	c2 := &connector{publicAddress: a2}

	// c1 should not have c2's address (a2)
	c1.merge(c2)
	assert.Equal(t, a2, c1.publicAddress)
}

func TestConnector_merge_connected(t *testing.T) {
	a := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}

	// when c1 is not connected and c2 is, c1 should get c2's clientConn
	c1 := &connector{
		publicAddress: a,
		clientConn:    nil,
	}
	c2 := &connector{
		publicAddress: a,
		clientConn:    &grpc.ClientConn{},
	}
	c1.merge(c2)
	assert.Equal(t, c2.clientConn, c1.clientConn)

	// when c2 is not connected and c1 is, c1 should keep it's clientConn
	c1 = &connector{
		publicAddress: a,
		clientConn:    &grpc.ClientConn{},
	}
	c2 = &connector{
		publicAddress: a,
		clientConn:    nil,
	}
	c1.merge(c2)
	assert.NotNil(t, c1.clientConn)

	// when neither is connected, there's nothing to do
	c1 = &connector{
		publicAddress: a,
		clientConn:    nil,
	}
	c2 = &connector{
		publicAddress: a,
		clientConn:    nil,
	}
	c1.merge(c2)
	assert.NotNil(t, c1.clientConn)

	// TODO (drausin) finish writing tests
}

type fixedDialer struct {
	clientConn *grpc.ClientConn
	dialErr    error
}

func (f *fixedDialer) Dial(addr *net.TCPAddr) (*grpc.ClientConn, error) {
	return f.clientConn, f.dialErr
}

/*
type fixedClientConnStateGetter struct {
	state connectivity.State
}

func (sg fixedClientConnStateGetter) get(cc *grpc.ClientConn) connectivity.State {
	return sg.state
}
*/
