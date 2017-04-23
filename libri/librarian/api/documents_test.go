package api

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/stretchr/testify/assert"
)

func TestGetKey(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	value, _ := NewTestDocument(rng)
	key, err := GetKey(value)
	assert.Nil(t, err)
	assert.Nil(t, ValidateBytes(key.Bytes(), DocumentKeyLength, "key"))
}

func TestGetAuthorPub(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	expected := ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng))

	entry := NewTestSinglePageEntry(rng)
	entry.AuthorPublicKey = expected
	assert.Equal(t, expected, GetAuthorPub(&Document{&Document_Entry{Entry: entry}}))

	entry = NewTestMultiPageEntry(rng)
	entry.AuthorPublicKey = expected
	assert.Equal(t, expected, GetAuthorPub(&Document{&Document_Entry{Entry: entry}}))

	page := NewTestPage(rng)
	page.AuthorPublicKey = expected
	assert.Equal(t, expected, GetAuthorPub(&Document{&Document_Page{Page: page}}))

	envelope := NewTestEnvelope(rng)
	envelope.AuthorPublicKey = expected
	assert.Equal(t, expected, GetAuthorPub(&Document{&Document_Envelope{Envelope: envelope}}))
}

func TestGetEntryPageKeys(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	singlePageDoc := &Document{
		Contents: &Document_Entry{
			Entry: NewTestSinglePageEntry(rng),
		},
	}
	pageKeys1, err := GetEntryPageKeys(singlePageDoc)
	assert.Nil(t, err)
	assert.Nil(t, pageKeys1)

	multiPageDoc := &Document{
		Contents: &Document_Entry{
			Entry: NewTestMultiPageEntry(rng),
		},
	}
	pageKeys2, err := GetEntryPageKeys(multiPageDoc)
	assert.Nil(t, err)
	assert.NotNil(t, len(pageKeys2) > 1)
}

func TestGetPageDocument(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	page := NewTestPage(rng)
	pageDoc, docKey, err := GetPageDocument(page)
	assert.Nil(t, err)
	assert.Equal(t, page, pageDoc.Contents.(*Document_Page).Page)
	assert.NotNil(t, docKey)

	pageDoc, docKey, err = GetPageDocument(nil)
	assert.NotNil(t, err)
	assert.Nil(t, pageDoc)
	assert.Nil(t, docKey)
}

func TestValidateDocument_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	d1 := &Document{&Document_Envelope{NewTestEnvelope(rng)}}
	assert.Nil(t, ValidateDocument(d1))

	d2, _ := NewTestDocument(rng)
	assert.Nil(t, ValidateDocument(d2))

	d3 := &Document{&Document_Page{NewTestPage(rng)}}
	assert.Nil(t, ValidateDocument(d3))
}

func TestValidateEnvelope_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	e := NewTestEnvelope(rng)
	assert.Nil(t, ValidateEnvelope(e))
}

func TestValidateEnvelope_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badLen := RandBytes(rng, 100)
	empty, zeros := []byte{}, []byte{0, 0, 0}

	cases := []func(e *Envelope){
		func(e *Envelope) { e.AuthorPublicKey = nil },    // 0) can't be nil
		func(e *Envelope) { e.AuthorPublicKey = empty },  // 1) can't be 0-length
		func(e *Envelope) { e.AuthorPublicKey = zeros },  // 2) can't be all zeros
		func(e *Envelope) { e.AuthorPublicKey = badLen }, // 3) length must be 65
		func(e *Envelope) { e.ReaderPublicKey = nil },    // 4) can't be nil
		func(e *Envelope) { e.ReaderPublicKey = empty },  // 5) can't be 0-length
		func(e *Envelope) { e.ReaderPublicKey = zeros },  // 6) can't be all zeros
		func(e *Envelope) { e.ReaderPublicKey = badLen }, // 7) length must be 65
		func(e *Envelope) { e.EntryKey = nil },           // 8) can't be nil
		func(e *Envelope) { e.EntryKey = empty },         // 9) can't be 0-length
		func(e *Envelope) { e.EntryKey = zeros },         // 10) can't be all zeros
		func(e *Envelope) { e.EntryKey = badLen },        // 11) length must be 65
	}

	assert.NotNil(t, ValidateEnvelope(nil))
	for i, c := range cases {
		badEnvelope := NewTestEnvelope(rng)
		c(badEnvelope)
		assert.NotNil(t, ValidateEnvelope(badEnvelope), fmt.Sprintf("case %d", i))
	}
}

func TestValidateEntry_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	e1 := NewTestSinglePageEntry(rng)
	assert.Nil(t, ValidateEntry(e1))

	e2 := NewTestSinglePageEntry(rng)
	e2.Contents = &Entry_PageKeys{
		&PageKeys{
			Keys: [][]byte{[]byte{0, 1, 2}, []byte{1, 2, 3}},
		},
	}
	assert.Nil(t, ValidateEntry(e2))
}

func TestValidateEntry_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badLen, diffPK := RandBytes(rng, 100), fakePubKey(rng)
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
		func(e *Entry) { e.AuthorPublicKey = diffPK },       // 12) different PK from Page
	}

	assert.NotNil(t, ValidateEntry(nil))
	for i, c := range cases {
		badEntry := NewTestSinglePageEntry(rng)
		c(badEntry)
		assert.NotNil(t, ValidateEntry(badEntry), fmt.Sprintf("case %d", i))
	}
}

func TestValidatePage_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p := NewTestPage(rng)
	assert.Nil(t, ValidatePage(p))
}

func TestValidatePage_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	badLen := RandBytes(rng, 100)
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
		badPage := NewTestPage(rng)
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

func TestValidatePublicKey(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, ValidatePublicKey(RandBytes(rng, 65)))
	assert.NotNil(t, ValidatePublicKey(RandBytes(rng, 32)))
	assert.NotNil(t, ValidatePublicKey(make([]byte, 0)))
	assert.NotNil(t, ValidatePublicKey(make([]byte, 65)))
	assert.NotNil(t, ValidatePublicKey(nil))
}

func TestValidateAESKey(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, ValidateAESKey(RandBytes(rng, 32)))
	assert.NotNil(t, ValidateAESKey(RandBytes(rng, 16)))
	assert.NotNil(t, ValidateAESKey(make([]byte, 0)))
	assert.NotNil(t, ValidateAESKey(make([]byte, 32)))
	assert.NotNil(t, ValidateAESKey(nil))
}

func TestValidateHMACKey(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, ValidateHMACKey(RandBytes(rng, 32)))
	assert.NotNil(t, ValidateHMACKey(RandBytes(rng, 16)))
	assert.NotNil(t, ValidateHMACKey(make([]byte, 0)))
	assert.NotNil(t, ValidateHMACKey(make([]byte, 32)))
	assert.NotNil(t, ValidateHMACKey(nil))
}

func TestValidatePageIVSeed(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, ValidatePageIVSeed(RandBytes(rng, 32)))
	assert.NotNil(t, ValidatePageIVSeed(RandBytes(rng, 16)))
	assert.NotNil(t, ValidatePageIVSeed(make([]byte, 0)))
	assert.NotNil(t, ValidatePageIVSeed(make([]byte, 32)))
	assert.NotNil(t, ValidatePageIVSeed(nil))
}

func TestValidateMetadataIV(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, ValidateMetadataIV(RandBytes(rng, 12)))
	assert.NotNil(t, ValidateMetadataIV(RandBytes(rng, 32)))
	assert.NotNil(t, ValidateMetadataIV(make([]byte, 0)))
	assert.NotNil(t, ValidateMetadataIV(make([]byte, 12)))
	assert.NotNil(t, ValidateMetadataIV(nil))
}

func TestValidateHMAC256(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, ValidateHMAC256(RandBytes(rng, 32)))
	assert.NotNil(t, ValidateHMAC256(RandBytes(rng, 16)))
	assert.NotNil(t, ValidateHMAC256(make([]byte, 0)))
	assert.NotNil(t, ValidateHMAC256(make([]byte, 32)))
	assert.NotNil(t, ValidateHMAC256(nil))
}
