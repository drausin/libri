package ship

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"errors."
	"github.com/stretchr/testify/assert"
)

func TestReceiver_Receive_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	authorKeys, readerKeys := keychain.New(3), keychain.New(3)
	authorKey, err := authorKeys.Sample()
	assert.Nil(t, err)
	readerKey, err := readerKeys.Sample()
	assert.Nil(t, err)
	cb := &fixedClientBalancer{}

	entries := []*api.Document{
		{
			Contents: &api.Document_Entry{
				Entry: api.NewTestSinglePageEntry(rng),
			},
		},
		{
			Contents: &api.Document_Entry{
				Entry: api.NewTestMultiPageEntry(rng),
			},
		},
	}
	for _, entry := range entries {
		pageKeys, err := api.GetEntryPageKeys(entry)
		assert.Nil(t, err)
		entryKey, err := api.GetKey(entry)
		assert.Nil(t, err)
		envelope := pack.NewEnvelopeDoc(
			ecid.ToPublicKeyBytes(authorKey),
			ecid.ToPublicKeyBytes(readerKey),
			entryKey,
		)
		envelopeKey, err := api.GetKey(envelope)
		assert.Nil(t, err)
		acq := &fixedAcquirer{
			docs: make(map[string]*api.Document),
		}
		acq.docs[entryKey.String()] = entry
		acq.docs[envelopeKey.String()] = envelope
		msAcq := &fixedMultiStoreAcquirer{}
		docS := &fixedStorer{}
		r := NewReceiver(cb, readerKeys, acq, msAcq, docS)

		receivedEntry, receivedKeys, err := r.Receive(envelopeKey)
		assert.Nil(t, err)
		assert.Equal(t, entry, receivedEntry)
		assert.NotNil(t, receivedKeys)

		// check that pages have been stored, if necessary
		assert.Equal(t, pageKeys, msAcq.docKeys)
		switch entry.Contents.(*api.Document_Entry).Entry.Contents.(type) {
		case *api.Entry_PageKeys:
			// pages would have been stored on the MultiStoreAcquirer.Acquire(...)
			// call
			assert.Nil(t, docS.storedKey)
			assert.Nil(t, docS.storedValue)
		case *api.Entry_Page:
			assert.NotNil(t, docS.storedKey)
			assert.NotNil(t, docS.storedValue)
		}
	}
}

func TestReceiver_Receive_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}
	authorKeys, readerKeys := keychain.New(3), keychain.New(3)
	authorKey, err := authorKeys.Sample()
	assert.Nil(t, err)
	readerKey, err := readerKeys.Sample()
	assert.Nil(t, err)
	acq := &fixedAcquirer{
		docs: make(map[string]*api.Document),
	}
	msAcq := &fixedMultiStoreAcquirer{}
	docS := &fixedStorer{}
	entry, entryKey := api.NewTestDocument(rng)
	envelope := pack.NewEnvelopeDoc(
		ecid.ToPublicKeyBytes(authorKey),
		ecid.ToPublicKeyBytes(readerKey),
		entryKey,
	)
	envelopeKey, err := api.GetKey(envelope)
	assert.Nil(t, err)
	acq.docs[envelopeKey.String()] = envelope

	// check clientBalancer.Next() error bubbles up
	cb1 := &fixedClientBalancer{errors.New("some Next error")}
	r1 := NewReceiver(cb1, readerKeys, acq, msAcq, docS)
	receivedDoc, receivedKeys, err := r1.Receive(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check acquire error bubbles up
	acq2 := &fixedAcquirer{err: errors.New("some Acquire error")}
	r2 := NewReceiver(cb, readerKeys, acq2, msAcq, docS)
	receivedDoc, receivedKeys, err = r2.Receive(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check SeparateEnvelopeDoc error bubbles up
	acq3 := &fixedAcquirer{docs: make(map[string]*api.Document)}
	acq3.docs[envelopeKey.String()] = entry // wrong doc type
	r3 := NewReceiver(cb, readerKeys, acq3, msAcq, docS)
	receivedDoc, receivedKeys, err = r3.Receive(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check createEncryptionKeys error bubbles up
	//
	// readerKeys4 will cause createEncryptionKeys to fail b/c can't find readerKey
	// in the different keychain
	readerKeys4 := keychain.New(1)
	r4 := NewReceiver(cb, readerKeys4, acq, msAcq, docS)
	receivedDoc, receivedKeys, err = r4.Receive(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check error from Acquire on entryKey bubbles up
	//
	// acq5 doesn't have entryKey, which will trigger error
	acq5 := &fixedAcquirer{docs: make(map[string]*api.Document)}
	acq5.docs[envelopeKey.String()] = envelope
	r5 := NewReceiver(cb, readerKeys, acq5, msAcq, docS)
	receivedDoc, receivedKeys, err = r5.Receive(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check getPages error bubbles up
	acq6 := &fixedAcquirer{docs: make(map[string]*api.Document)}
	acq6.docs[envelopeKey.String()] = envelope
	acq6.docs[entryKey.String()] = envelope // wrong doc type
	r6 := NewReceiver(cb, readerKeys, acq6, msAcq, docS)
	receivedDoc, receivedKeys, err = r6.Receive(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)
}

func TestReceiver_createEncryptionKeys_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}
	acq := &fixedAcquirer{}
	msAcq := &fixedMultiStoreAcquirer{}
	docS := &fixedStorer{}

	// check readerKeys.Get() error bubbles up
	readerKeys1 := &fixedKeychain{in: false}
	r1 := NewReceiver(cb, readerKeys1, acq, msAcq, docS).(*receiver)
	keys, err := r1.createEncryptionKeys(nil, nil)
	assert.Equal(t, keychain.ErrUnexpectedMissingKey, err)
	assert.Nil(t, keys)

	// check ecid.FromPublicKeyButes error bubbles up
	readerKeys2 := &fixedKeychain{in: true} // allows us to not err on readerKeys.Get()
	r2 := NewReceiver(cb, readerKeys2, acq, msAcq, docS).(*receiver)
	keys, err = r2.createEncryptionKeys(api.RandBytes(rng, 16), nil) // bad authorPubBytes
	assert.Equal(t, ecid.ErrKeyPointOffCurve, err)
	assert.Nil(t, keys)

	// check enc.NewKeys() error bubbles up
	wrongCurveKey, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	readerKeys3 := &fixedKeychain{getKey: ecid.FromPrivateKey(wrongCurveKey)}
	wrongCurveKeyPubBytes := ecid.ToPublicKeyBytes(readerKeys3.getKey)
	keys, err = r2.createEncryptionKeys(wrongCurveKeyPubBytes, nil)
	assert.Equal(t, ecid.ErrKeyPointOffCurve, err)
	assert.Nil(t, keys)
}

type fixedAcquirer struct {
	docs map[string]*api.Document
	err  error
}

func (f *fixedAcquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (
	*api.Document, error) {
	value, in := f.docs[docKey.String()]
	if !in {
		return nil, errors.New("missing")
	}
	return value, f.err
}

type fixedMultiStoreAcquirer struct {
	err       error
	docKeys   []id.ID
	authorPub []byte
}

func (f *fixedMultiStoreAcquirer) Acquire(
	docKeys []id.ID, authorPub []byte, cb api.ClientBalancer,
) error {
	f.docKeys, f.authorPub = docKeys, authorPub
	return f.err
}

type fixedStorer struct {
	err         error
	storedKey   id.ID
	storedValue *api.Document
}

func (f *fixedStorer) Store(key id.ID, value *api.Document) error {
	f.storedKey, f.storedValue = key, value
	return f.err
}

type fixedKeychain struct {
	getKey ecid.ID
	in     bool
}

func (f *fixedKeychain) Sample() (ecid.ID, error) {
	return nil, nil
}

func (f *fixedKeychain) Get(publicKey []byte) (ecid.ID, bool) {
	return f.getKey, f.in
}

func (f *fixedKeychain) Len() int {
	return 0
}
