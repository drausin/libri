package ship

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/rand"
	"testing"
	"errors"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/pack"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestReceiver_ReceiveEntry_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	authorKeys, readerKeys := keychain.New(3), keychain.New(3)
	authorKey, err := authorKeys.Sample()
	assert.Nil(t, err)
	readerKey, err := readerKeys.Sample()
	assert.Nil(t, err)
	kek, err := enc.NewKEK(authorKey.Key(), &readerKey.Key().PublicKey)
	assert.Nil(t, err)
	cb := &fixedGetterBalancer{}

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
	for _, entry1 := range entries {
		pageKeys, err := api.GetEntryPageKeys(entry1)
		assert.Nil(t, err)
		entryKey, err := api.GetKey(entry1)
		assert.Nil(t, err)
		eek1 := enc.NewPseudoRandomEEK(rng)
		eekCiphertext, eekCiphertextMAC, err := kek.Encrypt(eek1)
		assert.Nil(t, err)
		envelope := pack.NewEnvelopeDoc(
			entryKey,
			authorKey.PublicKeyBytes(),
			readerKey.PublicKeyBytes(),
			eekCiphertext,
			eekCiphertextMAC,
		)
		envelopeKey, err := api.GetKey(envelope)
		assert.Nil(t, err)
		acq := &fixedAcquirer{
			docs: make(map[string]*api.Document),
		}
		acq.docs[entryKey.String()] = entry1
		acq.docs[envelopeKey.String()] = envelope
		msAcq := &fixedMultiStoreAcquirer{}
		docS := &fixedStorer{}
		r := NewReceiver(cb, readerKeys, acq, msAcq, docS)

		entry2, eek2, err := r.ReceiveEntry(envelopeKey)
		assert.Nil(t, err)
		assert.Equal(t, entry1, entry2)
		assert.Equal(t, eek1, eek2)

		// check that pages have been stored, if necessary
		assert.Equal(t, pageKeys, msAcq.docKeys)
		switch entry1.Contents.(*api.Document_Entry).Entry.Contents.(type) {
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

func TestReceiver_ReceiveEntry_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedGetterBalancer{}
	authorKeys, readerKeys := keychain.New(3), keychain.New(3)
	authorKey, err := authorKeys.Sample()
	assert.Nil(t, err)
	readerKey, err := readerKeys.Sample()
	assert.Nil(t, err)
	kek, err := enc.NewKEK(authorKey.Key(), &readerKey.Key().PublicKey)
	assert.Nil(t, err)
	acq := &fixedAcquirer{
		docs: make(map[string]*api.Document),
	}
	msAcq := &fixedMultiStoreAcquirer{}
	docS := &fixedStorer{}
	entry, entryKey := api.NewTestDocument(rng)
	eek1 := enc.NewPseudoRandomEEK(rng)
	eekCiphertext, eekCiphertextMAC, err := kek.Encrypt(eek1)
	assert.Nil(t, err)
	envelope := pack.NewEnvelopeDoc(
		entryKey,
		authorKey.PublicKeyBytes(),
		readerKey.PublicKeyBytes(),
		eekCiphertext,
		eekCiphertextMAC,
	)
	envelopeKey, err := api.GetKey(envelope)
	assert.Nil(t, err)
	acq.docs[envelopeKey.String()] = envelope

	// check clientBalancer.Next() error bubbles up
	cb1 := &fixedGetterBalancer{err: errors.New("some Next error")}
	r1 := NewReceiver(cb1, readerKeys, acq, msAcq, docS)
	receivedDoc, receivedKeys, err := r1.ReceiveEntry(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check acquire error bubbles up
	acq2 := &fixedAcquirer{err: errors.New("some Acquire error")}
	r2 := NewReceiver(cb, readerKeys, acq2, msAcq, docS)
	receivedDoc, receivedKeys, err = r2.ReceiveEntry(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check wrong doc type error bubbles up
	acq3 := &fixedAcquirer{docs: make(map[string]*api.Document)}
	acq3.docs[envelopeKey.String()] = entry // wrong doc type
	r3 := NewReceiver(cb, readerKeys, acq3, msAcq, docS)
	receivedDoc, receivedKeys, err = r3.ReceiveEntry(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check GetEEK error bubbles up
	//
	// readerKeys4 will cause GetEEK to fail b/c can't find readerKey
	// in the different keychain
	readerKeys4 := keychain.New(1)
	r4 := NewReceiver(cb, readerKeys4, acq, msAcq, docS)
	receivedDoc, receivedKeys, err = r4.ReceiveEntry(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check error from Acquire on entryKey bubbles up
	//
	// acq5 doesn't have entryKey, which will trigger error
	acq5 := &fixedAcquirer{docs: make(map[string]*api.Document)}
	acq5.docs[envelopeKey.String()] = envelope
	r5 := NewReceiver(cb, readerKeys, acq5, msAcq, docS)
	receivedDoc, receivedKeys, err = r5.ReceiveEntry(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)

	// check getPages error bubbles up
	acq6 := &fixedAcquirer{docs: make(map[string]*api.Document)}
	acq6.docs[envelopeKey.String()] = envelope
	acq6.docs[entryKey.String()] = envelope // wrong doc type
	r6 := NewReceiver(cb, readerKeys, acq6, msAcq, docS)
	receivedDoc, receivedKeys, err = r6.ReceiveEntry(envelopeKey)
	assert.NotNil(t, err)
	assert.Nil(t, receivedDoc)
	assert.Nil(t, receivedKeys)
}

func TestReceiver_GetEEK_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedGetterBalancer{}
	acq := &fixedAcquirer{}
	msAcq := &fixedMultiStoreAcquirer{}
	docS := &fixedStorer{}

	// check readerKeys.Get() error bubbles up
	readerKeys1 := &fixedKeychain{in: false}
	r1 := NewReceiver(cb, readerKeys1, acq, msAcq, docS).(*receiver)
	env1 := &api.Envelope{}
	eek, err := r1.GetEEK(env1)
	assert.Equal(t, keychain.ErrUnexpectedMissingKey, err)
	assert.Nil(t, eek)

	// check ecid.FromPublicKeyButes error bubbles up
	readerKeys2 := &fixedKeychain{in: true} // allows us to not err on readerKeys.Get()
	r2 := NewReceiver(cb, readerKeys2, acq, msAcq, docS).(*receiver)
	env2 := &api.Envelope{
		AuthorPublicKey: api.RandBytes(rng, 16), // bad authorPubBytes
	}
	eek, err = r2.GetEEK(env2)
	assert.Equal(t, ecid.ErrKeyPointOffCurve, err)
	assert.Nil(t, eek)

	// check enc.NewKEK() error bubbles up
	wrongCurveKey, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	readerKeys3 := &fixedKeychain{
		getKey: ecid.FromPrivateKey(wrongCurveKey),
		in:     true,
	}
	wrongCurveKeyPubBytes := readerKeys3.getKey.PublicKeyBytes()
	env3 := &api.Envelope{
		AuthorPublicKey: wrongCurveKeyPubBytes,
	}
	r3 := NewReceiver(cb, readerKeys3, acq, msAcq, docS).(*receiver)
	eek, err = r3.GetEEK(env3)
	assert.Equal(t, ecid.ErrKeyPointOffCurve, err)
	assert.Nil(t, eek)

	// check Decrypt error bubbles up
	authorKeys4, readerKeys4 := keychain.New(1), keychain.New(1)
	authorKey, err := authorKeys4.Sample()
	assert.Nil(t, err)
	readerKey, err := readerKeys4.Sample()
	assert.Nil(t, err)
	env4 := &api.Envelope{
		AuthorPublicKey:  authorKey.PublicKeyBytes(),
		ReaderPublicKey:  readerKey.PublicKeyBytes(),
		EekCiphertext:    api.RandBytes(rng, api.EEKLength),
		EekCiphertextMac: api.RandBytes(rng, api.HMAC256Length), // does't match ciphertext
	}
	r4 := NewReceiver(cb, readerKeys4, acq, msAcq, docS).(*receiver)
	eek, err = r4.GetEEK(env4)
	assert.Equal(t, enc.ErrUnexpectedCiphertextMAC, err)
	assert.Nil(t, eek)
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
	docKeys []id.ID, authorPub []byte, cb api.GetterBalancer,
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
