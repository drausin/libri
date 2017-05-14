package api

import (
	"math/rand"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
)

// NewTestDocument generates a dummy Entry document for use in testing.
func NewTestDocument(rng *rand.Rand) (*Document, cid.ID) {
	doc := &Document{&Document_Entry{NewTestSinglePageEntry(rng)}}
	key, err := GetKey(doc)
	if err != nil {
		panic(err)
	}
	return doc, key
}

// NewTestEnvelope generates a dummy Envelope document for use in testing.
func NewTestEnvelope(rng *rand.Rand) *Envelope {
	return &Envelope{
		AuthorPublicKey: fakePubKey(rng),
		ReaderPublicKey: fakePubKey(rng),
		EntryKey:        RandBytes(rng, 32),
	}
}

// NewTestSinglePageEntry generates a dummy Entry document with a single Page for use in testing.
func NewTestSinglePageEntry(rng *rand.Rand) *Entry {
	page := NewTestPage(rng)
	return &Entry{
		AuthorPublicKey:       page.AuthorPublicKey,
		CreatedTime:           1,
		MetadataCiphertextMac: RandBytes(rng, 32),
		MetadataCiphertext:    RandBytes(rng, 64),
		Contents:              &Entry_Page{page},
	}
}

// NewTestMultiPageEntry generates a dummy Entry document with two page keys for use in testing.
func NewTestMultiPageEntry(rng *rand.Rand) *Entry {
	pageKeys := [][]byte{
		cid.NewPseudoRandom(rng).Bytes(),
		cid.NewPseudoRandom(rng).Bytes(),
	}
	return &Entry{
		AuthorPublicKey:       ecid.ToPublicKeyBytes(ecid.NewPseudoRandom(rng)),
		CreatedTime:           1,
		MetadataCiphertextMac: RandBytes(rng, 32),
		MetadataCiphertext:    RandBytes(rng, 64),
		Contents:              &Entry_PageKeys{PageKeys: &PageKeys{Keys: pageKeys}},
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

// NewTestPublication generates a dummy Publication for use in testing.
func NewTestPublication(rng *rand.Rand) *Publication {
	return &Publication{
		EnvelopeKey:     RandBytes(rng, cid.Length),
		EntryKey:        RandBytes(rng, cid.Length),
		AuthorPublicKey: fakePubKey(rng),
		ReaderPublicKey: fakePubKey(rng),
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
	return RandBytes(rng, ECPubKeyLength)
}
