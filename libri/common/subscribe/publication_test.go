package subscribe

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestNewRecentPublications_ok(t *testing.T) {
	rp, err := NewRecentPublications(uint32(2))
	assert.Nil(t, err)
	assert.NotNil(t, rp)
}

func TestNewRecentPublications_err(t *testing.T) {
	rp, err := NewRecentPublications(uint32(0))
	assert.NotNil(t, err)
	assert.Nil(t, rp)
}

func TestRecentPublications_Add(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value1 := api.NewTestPublication(rng)
	value2 := api.NewTestPublication(rng)
	value3 := api.NewTestPublication(rng)
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	key2, err := api.GetKey(value2)
	assert.Nil(t, err)
	key3, err := api.GetKey(value3)
	assert.Nil(t, err)
	fromPub1 := api.RandBytes(rng, api.ECPubKeyLength)
	fromPub2 := api.RandBytes(rng, api.ECPubKeyLength)
	fromPub3 := api.RandBytes(rng, api.ECPubKeyLength)
	rp, err := NewRecentPublications(uint32(2))
	assert.Nil(t, err)

	// value1 shouldn't be in cache
	pvr1, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub1)
	assert.Nil(t, err)
	in := rp.Add(pvr1)
	assert.False(t, in)

	// value1 now should be in cache
	pvr2, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub2)
	assert.Nil(t, err)
	in = rp.Add(pvr2)
	assert.True(t, in)

	// value 2 shouldn't be in cache
	pvr3, err := newPublicationValueReceipt(key2.Bytes(), value2, fromPub1)
	assert.Nil(t, err)
	in = rp.Add(pvr3)
	assert.False(t, in)

	// value 3 shouldn't be in cache
	pvr4, err := newPublicationValueReceipt(key3.Bytes(), value3, fromPub2)
	assert.Nil(t, err)
	in = rp.Add(pvr4)
	assert.False(t, in)

	// value1 should have been ejected on value3 add and so shouldn't currently be in cache
	pvr5, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub3)
	assert.Nil(t, err)
	in = rp.Add(pvr5)
	assert.False(t, in)
}

func TestRecentPublications_Get(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value := api.NewTestPublication(rng)
	key, err := api.GetKey(value)
	assert.Nil(t, err)
	fromPub := api.RandBytes(rng, api.ECPubKeyLength)
	rp, err := NewRecentPublications(uint32(2))
	assert.Nil(t, err)

	// check value not in the cache
	prs, in := rp.Get(key)
	assert.False(t, in)
	assert.Nil(t, prs)

	// add value
	pvr1, err := newPublicationValueReceipt(key.Bytes(), value, fromPub)
	assert.Nil(t, err)
	in = rp.Add(pvr1)
	assert.False(t, in)

	// check value in the cache
	prs, in = rp.Get(key)
	assert.True(t, in)
	assert.Equal(t, value, prs.Value)
}

func TestRecentPublications_Len(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value1 := api.NewTestPublication(rng)
	value2 := api.NewTestPublication(rng)
	key1, err := api.GetKey(value1)
	assert.Nil(t, err)
	key2, err := api.GetKey(value2)
	assert.Nil(t, err)
	fromPub1 := api.RandBytes(rng, api.ECPubKeyLength)
	fromPub2 := api.RandBytes(rng, api.ECPubKeyLength)
	rp, err := NewRecentPublications(uint32(2))
	assert.Nil(t, err)

	// check adding receipt for new value increments length
	pvr1, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub1)
	assert.Nil(t, err)
	rp.Add(pvr1)
	assert.Equal(t, 1, rp.Len())

	// check adding receipt for existing value does not increment length
	pvr2, err := newPublicationValueReceipt(key1.Bytes(), value1, fromPub2)
	assert.Nil(t, err)
	rp.Add(pvr2)
	assert.Equal(t, 1, rp.Len())

	// check adding receipt for new value increments length
	pvr3, err := newPublicationValueReceipt(key2.Bytes(), value2, fromPub1)
	assert.Nil(t, err)
	rp.Add(pvr3)
	assert.Equal(t, 2, rp.Len())
}

func TestNewPublicationValueReceipt_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value := api.NewTestPublication(rng)
	key, err := api.GetKey(value)
	assert.Nil(t, err)
	fromPub := api.RandBytes(rng, api.ECPubKeyLength)

	pvr, err := newPublicationValueReceipt(key.Bytes(), value, fromPub)
	assert.Nil(t, err)
	assert.Equal(t, value, pvr.pub.Value)
	assert.Equal(t, key, pvr.pub.Key)
	assert.Equal(t, fromPub, pvr.receipt.FromPub)
	assert.NotNil(t, pvr.receipt.Time)
}

func TestNewPublicationValueReceipt_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value := api.NewTestPublication(rng)
	key, err := api.GetKey(value)
	assert.Nil(t, err)
	fromPub := api.RandBytes(rng, api.ECPubKeyLength)

	// check GetKey error bubbles up
	pvr, err := newPublicationValueReceipt(nil, value, fromPub)
	assert.NotNil(t, err)
	assert.Nil(t, pvr)

	// check bad key throws error
	pvr, err = newPublicationValueReceipt(api.RandBytes(rng, id.Length), value, fromPub)
	assert.Equal(t, api.ErrUnexpectedKey, err)
	assert.Nil(t, pvr)

	// check bad fromPub throws error
	pvr, err = newPublicationValueReceipt(key.Bytes(), value, nil)
	assert.NotNil(t, err)
	assert.Nil(t, pvr)
}
