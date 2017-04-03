package api

import (
	"testing"
	"math/rand"
	"github.com/stretchr/testify/assert"
)

func TestValidateMetadata_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)
	assert.NotNil(t, m)
}

func TestValidateMetadata_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"

	m1, err := NewEntryMetadata("", 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Equal(t, UnexpectedZeroErr, err)
	assert.Nil(t, m1)

	m2, err := NewEntryMetadata(mediaType, 0, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Equal(t, UnexpectedZeroErr, err)
	assert.Nil(t, m2)

	m3, err := NewEntryMetadata(mediaType, 1, nil, 2, randBytes(rng, 32))
	assert.NotNil(t, err)
	assert.Nil(t, m3)

	m4, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 0, randBytes(rng, 32))
	assert.Equal(t, UnexpectedZeroErr, err)
	assert.Nil(t, m4)

	m5, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, nil)
	assert.NotNil(t, err)
	assert.Nil(t, m5)
}

func TestMetadata_GetMediaType(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)
	value, in := m.GetMediaType()
	assert.Equal(t, mediaType, value)
	assert.True(t, in)
}

func TestMetadata_GetCiphertextSize(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)
	value, in := m.GetCiphertextSize()
	assert.Equal(t, uint64(1), value)
	assert.True(t, in)
}

func TestMetadata_GetCiphertextMAC(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType, ciphertextMAC := "application/x-pdf", randBytes(rng, 32)
	m, err := NewEntryMetadata(mediaType, 1, ciphertextMAC, 2, randBytes(rng, 32))
	assert.Nil(t, err)
	value, in := m.GetCiphertextMAC()
	assert.Equal(t, ciphertextMAC, value)
	assert.True(t, in)
}

func TestMetadata_GetUncompressedSize(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)
	value, in := m.GetUncompressedSize()
	assert.Equal(t, uint64(2), value)
	assert.True(t, in)
}

func TestMetadata_GetUncompressedMAC(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType, uncompressedMAC := "application/x-pdf", randBytes(rng, 32)
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, uncompressedMAC)
	assert.Nil(t, err)
	value, in := m.GetUncompressedMAC()
	assert.Equal(t, uncompressedMAC, value)
	assert.True(t, in)
}

func TestSetGetBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)

	key := "some key"
	value, in := m.GetBytes(key)
	assert.Zero(t, value)
	assert.False(t, in)

	bytesValue := randBytes(rng, 16)
	m.SetBytes(key, bytesValue)

	value, in = m.GetBytes(key)
	assert.Equal(t, bytesValue, value)
	assert.True(t, in)
}

func TestSetGetString(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)

	key := "some key"
	value, in := m.GetString(key)
	assert.Zero(t, value)
	assert.False(t, in)

	stringValue := "some string value"
	m.SetString(key, stringValue)

	value, in = m.GetString(key)
	assert.Equal(t, stringValue, value)
	assert.True(t, in)
}

func TestSetGetUint64(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	mediaType := "application/x-pdf"
	m, err := NewEntryMetadata(mediaType, 1, randBytes(rng, 32), 2, randBytes(rng, 32))
	assert.Nil(t, err)

	key := "some key"
	value, in := m.GetUint64(key)
	assert.Zero(t, value)
	assert.False(t, in)

	uint64Value := uint64(1)
	m.SetUint64(key, uint64Value)

	value, in = m.GetUint64(key)
	assert.Equal(t, uint64Value, value)
	assert.True(t, in)
}
