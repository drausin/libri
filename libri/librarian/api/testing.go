package api

import (
	"math/rand"
	cid "github.com/drausin/libri/libri/common/id"
)

func NewTestDocument(rng *rand.Rand) (*Document, cid.ID) {
	doc := &Document{&Document_Entry{NewTestPageEntry(rng)}}
	key, err := GetKey(doc)
	if err != nil {
		panic(err)
	}
	return doc, key
}

func NewTestEnvelope(rng *rand.Rand) *Envelope {
	return &Envelope{
		AuthorPublicKey:          fakePubKey(rng),
		ReaderPublicKey:          fakePubKey(rng),
		EntryKey:                 randBytes(rng, 32),
		EncryptionKeysCiphertext: randBytes(rng, 108),
	}
}

func NewTestPageEntry(rng *rand.Rand) *Entry {
	page := NewTestPage(rng)
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

func NewTestPage(rng *rand.Rand) *Page {
	return &Page{
		AuthorPublicKey: fakePubKey(rng),
		CiphertextMac:   randBytes(rng, 32),
		Ciphertext:      randBytes(rng, 64),
	}
}

func fakePubKey(rng *rand.Rand) []byte {
	return randBytes(rng, 65)
}

func randBytes(rng *rand.Rand, length int) []byte {
	b := make([]byte, length)
	_, err := rng.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}