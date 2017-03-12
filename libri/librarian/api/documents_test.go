package api

import (
	"fmt"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

type Document_BadContent struct{}

func (*Document_BadContent) isDocument_Contents() {}

func TestValidateDocument_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	d1 := &Document{&Document_Envelope{newTestEnvelope(rng)}}
	assert.Nil(t, ValidateDocument(d1))
	d2 := &Document{&Document_Entry{newTestPageEntry(rng)}}
	assert.Nil(t, ValidateDocument(d2))
	d3 := &Document{&Document_Page{newTestPage(rng)}}
	assert.Nil(t, ValidateDocument(d3))
}

func TestValidateDocument_err(t *testing.T) {
	assert.NotNil(t, ValidateDocument(nil))
	d := &Document{&Document_BadContent{}}
	assert.NotNil(t, ValidateDocument(d))
}

func TestValidateEnvelope_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	e := newTestEnvelope(rng)
	assert.Nil(t, ValidateEnvelope(e))
}

func TestValidateEnvelope_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badLen := randBytes(rng, 100)
	empty, zeros := []byte{}, []byte{0, 0, 0}

	cases := []func(e *Envelope){
		func(e *Envelope) { e.AuthorPublicKey = nil },             // 0) can't be nil
		func(e *Envelope) { e.AuthorPublicKey = empty },           // 1) can't be 0-length
		func(e *Envelope) { e.AuthorPublicKey = zeros },           // 2) can't be all zeros
		func(e *Envelope) { e.AuthorPublicKey = badLen },          // 3) length must be 65
		func(e *Envelope) { e.ReaderPublicKey = nil },             // 4) can't be nil
		func(e *Envelope) { e.ReaderPublicKey = empty },           // 5) can't be 0-length
		func(e *Envelope) { e.ReaderPublicKey = zeros },           // 6) can't be all zeros
		func(e *Envelope) { e.ReaderPublicKey = badLen },          // 7) length must be 65
		func(e *Envelope) { e.EntryKey = nil },                    // 8) can't be nil
		func(e *Envelope) { e.EntryKey = empty },                  // 9) can't be 0-length
		func(e *Envelope) { e.EntryKey = zeros },                  // 10) can't be all zeros
		func(e *Envelope) { e.EntryKey = badLen },                 // 11) length must be 65
		func(e *Envelope) { e.EncryptionKeysCiphertext = nil },    // 12) can't be nil
		func(e *Envelope) { e.EncryptionKeysCiphertext = empty },  // 13) can't be 0-length
		func(e *Envelope) { e.EncryptionKeysCiphertext = zeros },  // 14) can't be all zeros
		func(e *Envelope) { e.EncryptionKeysCiphertext = badLen }, // 15) length must be 65
	}

	assert.NotNil(t, ValidateEnvelope(nil))
	for i, c := range cases {
		badEnvelope := newTestEnvelope(rng)
		c(badEnvelope)
		assert.NotNil(t, ValidateEnvelope(badEnvelope), fmt.Sprintf("case %d", i))
	}
}

func TestValidateEntry_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	e1 := newTestPageEntry(rng)
	assert.Nil(t, ValidateEntry(e1))

	e2 := newTestPageEntry(rng)
	e2.Contents = &Entry_PageKeys{
		&PageKeys{
			Keys: [][]byte{[]byte{0, 1, 2}, []byte{1, 2, 3}},
		},
	}
	assert.Nil(t, ValidateEntry(e2))
}

type Entry_BadContent struct{}

func (*Entry_BadContent) isEntry_Contents() {}

func TestValidateEntry_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badLen, diffPK := randBytes(rng, 100), ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))
	empty, zeros := []byte{}, []byte{0, 0, 0}

	// each of the cases takes a valid *Entry and changes it in some way to make it invalid
	cases := []func(e *Entry){
		func(e *Entry) { e.AuthorPublicKey = nil },          // 0) can't be nil
		func(e *Entry) { e.AuthorPublicKey = empty },        // 1) can't be zero-length
		func(e *Entry) { e.AuthorPublicKey = zeros },        // 2) can't be all zeros
		func(e *Entry) { e.AuthorPublicKey = badLen },       // 3) length must be 65
		func(e *Entry) { e.CreatedTime = 0 },                // 4) must be non-zero
		func(e *Entry) { e.MetadataCiphertextMac = nil },    // 5) can't be nil
		func(e *Entry) { e.MetadataCiphertextMac = empty },  // 6) can't be zero-length
		func(e *Entry) { e.MetadataCiphertextMac = zeros },  // 7) can't be all zeros
		func(e *Entry) { e.MetadataCiphertextMac = badLen }, // 8) length must be 65
		func(e *Entry) { e.MetadataCiphertext = nil },       // 9) can't be nil
		func(e *Entry) { e.MetadataCiphertext = empty },     // 10) can't be zero-length
		func(e *Entry) { e.MetadataCiphertext = zeros },     // 11) can't be all zeros
		func(e *Entry) { e.ContentsCiphertextMac = nil },    // 12) can't be nil
		func(e *Entry) { e.ContentsCiphertextMac = empty },  // 13) can't be zero-length
		func(e *Entry) { e.ContentsCiphertextMac = zeros },  // 14) can't be all zeros
		func(e *Entry) { e.ContentsCiphertextMac = badLen }, // 15) length must be 65
		func(e *Entry) { e.ContentsCiphertextSize = 0 },     // 16) must be non-zero
		func(e *Entry) { e.Contents = &Entry_BadContent{} }, // 17) bad content
		func(e *Entry) { e.AuthorPublicKey = diffPK },       // 18) different PK from Page

	}

	assert.NotNil(t, ValidateEntry(nil))
	for i, c := range cases {
		badEntry := newTestPageEntry(rng)
		c(badEntry)
		assert.NotNil(t, ValidateEntry(badEntry), fmt.Sprintf("case %d", i))
	}
}

func TestValidatePage_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p := newTestPage(rng)
	assert.Nil(t, ValidatePage(p))
}

func TestValidatePage_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badLen := randBytes(rng, 100)
	empty, zeros := []byte{}, []byte{0, 0, 0}

	// each of the cases takes a valid *Page and changes it in some way to make it invalid
	cases := []func(p *Page){
		func(p *Page) { p.AuthorPublicKey = nil },    // 0) can't be nil
		func(p *Page) { p.AuthorPublicKey = empty },  // 1) can't be zero-length
		func(p *Page) { p.AuthorPublicKey = zeros },  // 2) can't be all zeros
		func(p *Page) { p.AuthorPublicKey = badLen }, // 3) length must be 65
		func(p *Page) { p.CiphertextMac = empty },    // 5) can't be zero-length
		func(p *Page) { p.CiphertextMac = zeros },    // 6) can't be all zeros
		func(p *Page) { p.CiphertextMac = badLen },   // 7) length must be 32
		func(p *Page) { p.Ciphertext = nil },         // 8) can't be nil
		func(p *Page) { p.Ciphertext = empty },       // 9) can't be zero-length
		func(p *Page) { p.Ciphertext = zeros },       // 10) can't be all zeros
	}

	assert.NotNil(t, ValidatePage(nil))
	for i, c := range cases {
		badPage := newTestPage(rng)
		c(badPage)
		assert.NotNil(t, ValidatePage(badPage), fmt.Sprintf("case %d", i))
	}
}

func TestValidatePageKeys_ok(t *testing.T) {
	pk := &PageKeys{
		Keys: [][]byte{[]byte{0, 1, 2}, []byte{1, 2, 3}},
	}
	assert.Nil(t, ValidatePageKeys(pk))
}

func TestValidatePageKeys_err(t *testing.T) {
	cases := []*PageKeys{
		nil,                                                // 0) nil PageKeys not allowed
		{Keys: nil},                                        // 1) nil Keys not allowed
		{Keys: [][]byte{}},                                 // 2) must have at least one key
		{Keys: [][]byte{nil, nil}},                         // 3) nil keys not allowed
		{Keys: [][]byte{[]byte{}, []byte{0, 1, 2}}},        // 4) keys cannot be empty
		{Keys: [][]byte{[]byte{0, 1, 2}, []byte{}}},        // 5) keys cannot be empty
		{Keys: [][]byte{[]byte{0, 0, 0}, []byte{0, 1, 2}}}, // 6) keys may not equal 0
		{Keys: [][]byte{[]byte{0, 1, 2}, []byte{0, 0, 0}}}, // 7) keys may not equal 0
	}
	for i, c := range cases {
		assert.NotNil(t, ValidatePageKeys(c), fmt.Sprintf("case %d", i))
	}
}

func newTestEnvelope(rng *rand.Rand) *Envelope {
	return &Envelope{
		AuthorPublicKey:          ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng)),
		ReaderPublicKey:          ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng)),
		EntryKey:                 randBytes(rng, 32),
		EncryptionKeysCiphertext: randBytes(rng, 108),
	}
}

func newTestPageEntry(rng *rand.Rand) *Entry {
	page := newTestPage(rng)
	return &Entry{
		AuthorPublicKey:        page.AuthorPublicKey,
		CreatedTime:            1,
		MetadataCiphertextMac:  randBytes(rng, 32),
		MetadataCiphertext:     randBytes(rng, 64),
		ContentsCiphertextMac:  randBytes(rng, 32),
		ContentsCiphertextSize: 100,
		Contents:               &Entry_Page{page},
	}
}

func newTestPage(rng *rand.Rand) *Page {
	return &Page{
		AuthorPublicKey: ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng)),
		CiphertextMac:   randBytes(rng, 32),
		Ciphertext:      randBytes(rng, 64),
	}
}

func randBytes(rng *rand.Rand, length int) []byte {
	b := make([]byte, length)
	_, err := rng.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
