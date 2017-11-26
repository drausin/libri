package api

import (
	"math/rand"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
)

// NewTestDocument generates a dummy Entry document for use in testing.
func NewTestDocument(rng *rand.Rand) (*Document, id.ID) {
	doc := &Document{&Document_Entry{NewTestSinglePageEntry(rng)}}
	key, err := GetKey(doc)
	errors.MaybePanic(err)
	return doc, key
}

// NewTestEnvelope generates a dummy Envelope document for use in testing.
func NewTestEnvelope(rng *rand.Rand) *Envelope {
	return &Envelope{
		AuthorPublicKey:  fakePubKey(rng),
		ReaderPublicKey:  fakePubKey(rng),
		EntryKey:         RandBytes(rng, DocumentKeyLength),
		EekCiphertext:    RandBytes(rng, EEKCiphertextLength),
		EekCiphertextMac: RandBytes(rng, HMAC256Length),
	}
}

// NewTestSinglePageEntry generates a dummy Entry document with a single Page for use in testing.
func NewTestSinglePageEntry(rng *rand.Rand) *Entry {
	page := NewTestPage(rng)
	return &Entry{
		AuthorPublicKey:       page.AuthorPublicKey,
		CreatedTime:           1,
		MetadataCiphertext:    RandBytes(rng, 64),
		MetadataCiphertextMac: RandBytes(rng, HMAC256Length),
		Page: page,
	}
}

// NewTestMultiPageEntry generates a dummy Entry document with two page keys for use in testing.
func NewTestMultiPageEntry(rng *rand.Rand) *Entry {
	pageKeys := [][]byte{
		id.NewPseudoRandom(rng).Bytes(),
		id.NewPseudoRandom(rng).Bytes(),
	}
	return &Entry{
		AuthorPublicKey:       ecid.NewPseudoRandom(rng).PublicKeyBytes(),
		CreatedTime:           1,
		MetadataCiphertextMac: RandBytes(rng, 32),
		MetadataCiphertext:    RandBytes(rng, 64),
		PageKeys:              pageKeys,
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
		EnvelopeKey:     RandBytes(rng, id.Length),
		EntryKey:        RandBytes(rng, id.Length),
		AuthorPublicKey: fakePubKey(rng),
		ReaderPublicKey: fakePubKey(rng),
	}
}

// RandBytes generates a random bytes slice of a given length.
func RandBytes(rng *rand.Rand, length int) []byte {
	b := make([]byte, length)
	_, err := rng.Read(b)
	errors.MaybePanic(err)
	return b
}
func fakePubKey(rng *rand.Rand) []byte {
	return RandBytes(rng, ECPubKeyLength)
}
