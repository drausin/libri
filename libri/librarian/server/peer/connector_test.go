package peer

import (
	"net"
	"testing"

	"errors"

	"sync"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
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

func TestConnector_merge_otherNil(t *testing.T) {
	// when other is nil, c1 should keep its clientConn
	c1 := &connector{
		publicAddress: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100},
		clientConn:    &grpc.ClientConn{},
		mu:            new(sync.Mutex),
	}
	c1.merge(nil)
	assert.NotNil(t, c1.clientConn)

}
func TestConnector_merge_diffAddress(t *testing.T) {
	a1 := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20100}
	a2 := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20101}
	c1 := &connector{publicAddress: a1, mu: new(sync.Mutex)}
	c2 := &connector{publicAddress: a2, mu: new(sync.Mutex)}

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
		mu:            new(sync.Mutex),
	}
	c2 := &connector{
		publicAddress: a,
		clientConn:    &grpc.ClientConn{},
		mu:            new(sync.Mutex),
	}
	c1.merge(c2)
	assert.Equal(t, c2.clientConn, c1.clientConn)

	// when both c1 & c2 are connected, but only c2 is ready, c1 should get c2's connection
	cc1, cc2 := &grpc.ClientConn{}, &grpc.ClientConn{}
	c1 = &connector{
		publicAddress: a,
		clientConn:    cc1,
		stateGetter:   fixedClientConnStateGetter{connectivity.Idle},
		mu:            new(sync.Mutex),
	}
	c2 = &connector{
		publicAddress: a,
		clientConn:    cc2,
		stateGetter:   fixedClientConnStateGetter{connectivity.Ready},
		mu:            new(sync.Mutex),
	}
	c1.merge(c2)
	assert.Equal(t, cc2, c1.clientConn)
	// ideally would test that cc1 has been disconnected, but hard to do since grpc.ClientConn
	// is a struct and not an interface :(

	// when c2 is not connected and c1 is, c1 should keep it's clientConn
	c1 = &connector{
		publicAddress: a,
		clientConn:    &grpc.ClientConn{},
		mu:            new(sync.Mutex),
	}
	c2 = &connector{
		publicAddress: a,
		clientConn:    nil,
		mu:            new(sync.Mutex),
	}
	c1.merge(c2)
	assert.NotNil(t, c1.clientConn)

	// when neither is connected, there's nothing to do
	c1 = &connector{
		publicAddress: a,
		clientConn:    nil,
		mu:            new(sync.Mutex),
	}
	c2 = &connector{
		publicAddress: a,
		clientConn:    nil,
		mu:            new(sync.Mutex),
	}
	c1.merge(c2)
	assert.Nil(t, c1.clientConn)
}

type fixedDialer struct {
	clientConn *grpc.ClientConn
	dialErr    error
}

func (f *fixedDialer) Dial(addr *net.TCPAddr) (*grpc.ClientConn, error) {
	return f.clientConn, f.dialErr
}

type fixedClientConnStateGetter struct {
	state connectivity.State
}

func (sg fixedClientConnStateGetter) get(cc *grpc.ClientConn) connectivity.State {
	return sg.state
}
