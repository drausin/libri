package peer

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestTestConnector_Connect(t *testing.T) {
	c := &TestConnector{}
	client, err := c.Connect()
	assert.Nil(t, err)
	assert.Nil(t, client)
}

func TestTestConnector_Disconnect(t *testing.T) {
	c := &TestConnector{}
	err := c.Disconnect()
	assert.Nil(t, err)
}

func TestTestConnector_Address(t *testing.T) {
	c1 := &TestConnector{}
	assert.Nil(t, c1.Address())
}

func TestTestConnector_Connect_error(t *testing.T) {
	c := &TestConnector{ConnectErr: errors.New("some connect error")}
	client, err := c.Connect()
	assert.NotNil(t, err)
	assert.Nil(t, client)
}
