package client

import (
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestIntroducerCreator_Create_ok(t *testing.T) {
	ic := NewIntroducerCreator()
	_, err := ic.Create(&peer.TestConnector{Client: api.NewLibrarianClient(nil)})
	assert.Nil(t, err)
}
func TestIntroducerCreator_Create_err(t *testing.T) {
	ic := NewIntroducerCreator()
	_, err := ic.Create(&peer.TestConnector{ConnectErr: errors.New("some error")})
	assert.NotNil(t, err)
}

func TestFinderCreator_Create_ok(t *testing.T) {
	fc := NewFinderCreator()
	_, err := fc.Create(&peer.TestConnector{Client: api.NewLibrarianClient(nil)})
	assert.Nil(t, err)
}

func TestFinderCreator_Create_err(t *testing.T) {
	fc := NewFinderCreator()
	_, err := fc.Create(&peer.TestConnector{ConnectErr: errors.New("some error")})
	assert.NotNil(t, err)
}

func TestVerifyCreator_Create_ok(t *testing.T) {
	fc := NewVerifierCreator()
	_, err := fc.Create(&peer.TestConnector{Client: api.NewLibrarianClient(nil)})
	assert.Nil(t, err)
}

func TestVerifierCreator_Create_err(t *testing.T) {
	fc := NewVerifierCreator()
	_, err := fc.Create(&peer.TestConnector{ConnectErr: errors.New("some error")})
	assert.NotNil(t, err)
}

func TestStorerCreator_Create_ok(t *testing.T) {
	sc := NewStorerCreator()
	_, err := sc.Create(&peer.TestConnector{Client: api.NewLibrarianClient(nil)})
	assert.Nil(t, err)
}

func TestStorerCreator_Create_err(t *testing.T) {
	sc := NewStorerCreator()
	_, err := sc.Create(&peer.TestConnector{ConnectErr: errors.New("some error")})
	assert.NotNil(t, err)
}
