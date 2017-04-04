package api

import (
	"math/rand"

	cid "github.com/drausin/libri/libri/common/id"
)

// NewTestDocument generates a dummy Entry document for use in testing.
func NewTestDocument(rng *rand.Rand) (*Document, cid.ID) {
	doc := &Document{&Document_Entry{NewTestPageEntry(rng)}}
	key, err := GetKey(doc)
	if err != nil {
		panic(err)
	}
	return doc, key
}

// NewTestEnvelope generates a dummy Envelope document for use in testing.
func NewTestEnvelope(rng *rand.Rand) *Envelope {
	return &Envelope{
		AuthorPublicKey:          fakePubKey(rng),
		ReaderPublicKey:          fakePubKey(rng),
		EntryKey:                 RandBytes(rng, 32),
	}
}

// NewTestPageEntry generates a dummy Entry document with a single Page for use in testing.
func NewTestPageEntry(rng *rand.Rand) *Entry {
	page := NewTestPage(rng)
	return &Entry{
		AuthorPublicKey:        page.AuthorPublicKey,
		CreatedTime:            1,
		MetadataCiphertextMac:  RandBytes(rng, 32),
		MetadataCiphertext:     RandBytes(rng, 64),
		Contents:               &Entry_Page{page},
	}
}

// NewTestPage generates a dummy Page for use in testing.
func NewTestPage(rng *rand.Rand) *Page {
	return &Page{
		AuthorPublicKey: fakePubKey(rng),
		CiphertextMac:   RandBytes(rng, 32),
		Ciphertext:      RandBytes(rng, 64),
	}
}

// RandBytes generates a random bytes slice of a given length.
func RandBytes(rng *rand.Rand, length int) []byte {
	b := make([]byte, length)
	_, err := rng.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
func fakePubKey(rng *rand.Rand) []byte {
	return RandBytes(rng, 65)
}

