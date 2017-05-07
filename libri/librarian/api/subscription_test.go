package api

import (
	"fmt"
	"testing"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"github.com/drausin/libri/libri/common/id"
)

func TestValidateSubscription(t *testing.T) {
	err := ValidateSubscription(nil)
	assert.Equal(t, ErrUnexpectedNilValue, err)

	cases := []struct {
		authorPublicKeys *BloomFilter
		readerPublicKeys *BloomFilter
		valid bool
	} {
		{
			&BloomFilter{Encoded: []byte{1, 2, 3}},
			&BloomFilter{Encoded: []byte{4, 5, 6}},
			true,
		},
		{
			&BloomFilter{Encoded: []byte{1, 2, 3}},
			nil,
			false,
		},
		{
			&BloomFilter{Encoded: []byte{1, 2, 3}},
			&BloomFilter{Encoded: nil},
			false,
		},
		{
			nil,
			&BloomFilter{Encoded: []byte{4, 5, 6}},
			false,
		},
		{
			&BloomFilter{Encoded: nil},
			&BloomFilter{Encoded: []byte{4, 5, 6}},
			false,
		},
		{
			nil,
			nil,
			false,
		},
		{
			&BloomFilter{Encoded: nil},
			&BloomFilter{Encoded: nil},
			false,
		},
	}

	for i, c := range cases {
		info := fmt.Sprintf("i: %d", i)
		s := &Subscription{
			AuthorPublicKeys: c.authorPublicKeys,
			ReaderPublicKeys: c.readerPublicKeys,
		}
		err := ValidateSubscription(s)
		if c.valid {
			assert.Nil(t, err, info)
		} else {
			assert.NotNil(t, err, info)
		}
	}
}

func TestValidatePublication_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p := NewTestPublication(rng)
	err := ValidatePublication(p)
	assert.Nil(t, err)
}

func TestValidatePublication_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cases := []*Publication{
		nil,	// 0
		{},	// 1
		{	// 2
			EntryKey: nil,
			EnvelopeKey: RandBytes(rng, id.Length),
			AuthorPublicKey: fakePubKey(rng),
			ReaderPublicKey: fakePubKey(rng),
		},
		{	// 3
			EntryKey: RandBytes(rng, id.Length),
			EnvelopeKey: nil,
			AuthorPublicKey: fakePubKey(rng),
			ReaderPublicKey: fakePubKey(rng),
		},
		{	// 4
			EntryKey: RandBytes(rng, id.Length),
			EnvelopeKey: RandBytes(rng, id.Length),
			AuthorPublicKey: nil,
			ReaderPublicKey: fakePubKey(rng),
		},
		{	// 5
			EntryKey: RandBytes(rng, id.Length),
			EnvelopeKey: RandBytes(rng, id.Length),
			AuthorPublicKey: fakePubKey(rng),
			ReaderPublicKey: nil,
		},
	}
	for i, p := range cases {
		info := fmt.Sprintf("i: %d", i)
		assert.NotNil(t, ValidatePublication(p), info)
	}
}

func TestGetPublication_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	env := NewTestEnvelope(rng)
	doc := &Document{&Document_Envelope{Envelope: env}}
	key, err := GetKey(doc)
	assert.Nil(t, err)

	// check all fields are set correct
	p := GetPublication(key.Bytes(), doc)
	assert.Equal(t, env.EntryKey, p.EntryKey)
	assert.Equal(t, key.Bytes(), p.EnvelopeKey)
	assert.Equal(t, env.AuthorPublicKey, p.AuthorPublicKey)
	assert.Equal(t, env.ReaderPublicKey, p.ReaderPublicKey)

	// check non-envelope type has nil pub with no error
	doc, _ = NewTestDocument(rng)
	assert.Nil(t, err)
	p = GetPublication(key.Bytes(), doc)
	assert.Nil(t, err)
	assert.Nil(t, p)
}
