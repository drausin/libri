package peer

import (
	"testing"

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

func TestTestConnector_Equals(t *testing.T) {
	c1, c2 := &TestConnector{}, &TestConnector{}
	assert.False(t, c1.Equals(c2))
}

func TestTestConnector_String(t *testing.T) {
	c := &TestConnector{}
	assert.True(t, len(c.String()) > 0)
}

func TestTestErrConnector_Connect(t *testing.T) {
	c := &TestErrConnector{}
	client, err := c.Connect()
	assert.NotNil(t, err)
	assert.Nil(t, client)
}

func TestTestErrConnector_Disconnect(t *testing.T) {
	c := &TestErrConnector{}
	err := c.Disconnect()
	assert.Nil(t, err)
}
func TestTestErrConnector_Equals(t *testing.T) {
	c1, c2 := &TestErrConnector{}, &TestErrConnector{}
	assert.False(t, c1.Equals(c2))
}

func TestTestErrConnector_String(t *testing.T) {
	c := &TestErrConnector{}
	assert.True(t, len(c.String()) > 0)
}

