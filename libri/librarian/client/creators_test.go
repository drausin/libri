package client

import (
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestIntroducerCreator_Create_ok(t *testing.T) {
	p := &fixedPool{lc: api.NewLibrarianClient(nil), getAddresses: make(map[string]struct{})}
	ic := NewIntroducerCreator(p)
	i, err := ic.Create("some address")
	assert.Nil(t, err)
	assert.NotNil(t, i)
}
func TestIntroducerCreator_Create_err(t *testing.T) {
	p := &fixedPool{getErr: errors.New("some error"), getAddresses: make(map[string]struct{})}
	ic := NewIntroducerCreator(p)
	i, err := ic.Create("some address")
	assert.NotNil(t, err)
	assert.Nil(t, i)
}

func TestFinderCreator_Create_ok(t *testing.T) {
	p := &fixedPool{lc: api.NewLibrarianClient(nil), getAddresses: make(map[string]struct{})}
	fc := NewFinderCreator(p)
	f, err := fc.Create("some address")
	assert.Nil(t, err)
	assert.NotNil(t, f)
}

func TestFinderCreator_Create_err(t *testing.T) {
	p := &fixedPool{getErr: errors.New("some error"), getAddresses: make(map[string]struct{})}
	fc := NewFinderCreator(p)
	f, err := fc.Create("some address")
	assert.NotNil(t, err)
	assert.Nil(t, f)
}

func TestVerifyCreator_Create_ok(t *testing.T) {
	p := &fixedPool{lc: api.NewLibrarianClient(nil), getAddresses: make(map[string]struct{})}
	fc := NewVerifierCreator(p)
	v, err := fc.Create("some address")
	assert.Nil(t, err)
	assert.NotNil(t, v)
}

func TestVerifierCreator_Create_err(t *testing.T) {
	p := &fixedPool{getErr: errors.New("some error"), getAddresses: make(map[string]struct{})}
	fc := NewVerifierCreator(p)
	v, err := fc.Create("some address")
	assert.NotNil(t, err)
	assert.Nil(t, v)
}

func TestStorerCreator_Create_ok(t *testing.T) {
	p := &fixedPool{lc: api.NewLibrarianClient(nil), getAddresses: make(map[string]struct{})}
	sc := NewStorerCreator(p)
	s, err := sc.Create("some address")
	assert.Nil(t, err)
	assert.NotNil(t, s)
}

func TestStorerCreator_Create_err(t *testing.T) {
	p := &fixedPool{getErr: errors.New("some error"), getAddresses: make(map[string]struct{})}
	sc := NewStorerCreator(p)
	s, err := sc.Create("some address")
	assert.NotNil(t, err)
	assert.Nil(t, s)
}
