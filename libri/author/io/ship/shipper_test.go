package ship

import (
	"errors"
	"math/rand"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/stretchr/testify/assert"
)

func TestShipper_Ship_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	kek, authorPub, readerPub := enc.NewPseudoRandomKEK(rng)
	eek := enc.NewPseudoRandomEEK(rng)
	mlPub := &fixedMultiLoadPublisher{}
	s := NewShipper(
		&fixedPutterBalancer{},
		&fixedPublisher{},
		mlPub,
	)
	entry := &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestMultiPageEntry(rng),
		},
	}
	origEntryKey, err := api.GetKey(entry)
	assert.Nil(t, err)

	// test multi-page ship
	envelope, envelopeKey, err := s.ShipEntry(entry, authorPub, readerPub, kek, eek)
	assert.Nil(t, err)
	assert.NotNil(t, envelope)
	assert.NotNil(t, envelopeKey)
	assert.Equal(t, origEntryKey.Bytes(),
		envelope.Contents.(*api.Document_Envelope).Envelope.EntryKey)
	assert.True(t, mlPub.deleted)

	// test single-page ship
	entry = &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestSinglePageEntry(rng),
		},
	}
	origEntryKey, err = api.GetKey(entry)
	assert.Nil(t, err)
	envelope, envelopeKey, err = s.ShipEntry(entry, authorPub, readerPub, kek, eek)
	assert.Nil(t, err)
	assert.NotNil(t, envelope)
	assert.NotNil(t, envelopeKey)
	assert.Equal(t, origEntryKey.Bytes(),
		envelope.Contents.(*api.Document_Envelope).Envelope.EntryKey)
}

func TestShipper_Ship_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	kek, authorPub, readerPub := enc.NewPseudoRandomKEK(rng)
	eek := enc.NewPseudoRandomEEK(rng)
	entry := &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestMultiPageEntry(rng),
		},
	}

	s := NewShipper(
		&fixedPutterBalancer{},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{err: errors.New("some Publish error")},
	)

	// check GetEntryPageKeys error bubbles up
	envelope := &api.Document{ // wrong doc type
		Contents: &api.Document_Envelope{
			Envelope: api.NewTestEnvelope(rng),
		},
	}
	envelope, entryKey, err := s.ShipEntry(envelope, authorPub, readerPub, kek, eek)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	// check page publish error bubbles up
	envelope, entryKey, err = s.ShipEntry(entry, authorPub, readerPub, kek, eek)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	// check getting next librarian error bubbles up
	s = NewShipper(
		&fixedPutterBalancer{err: errors.New("some Next error")},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{},
	)
	envelope, entryKey, err = s.ShipEntry(entry, authorPub, readerPub, kek, eek)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	// check entry publish error bubbles up
	s = NewShipper(
		&fixedPutterBalancer{},
		&fixedPublisher{[]error{errors.New("some Publish error")}},
		&fixedMultiLoadPublisher{},
	)
	envelope, entryKey, err = s.ShipEntry(entry, authorPub, readerPub, kek, eek)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	// check kek.Encrypt error bubbles up
	s = NewShipper(
		&fixedPutterBalancer{},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{},
	)
	envelope, entryKey, err = s.ShipEntry(entry, authorPub, readerPub, &enc.KEK{}, eek)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	// check envelope publish error bubbles up
	s = NewShipper(
		&fixedPutterBalancer{},
		&fixedPublisher{[]error{nil, errors.New("some Publish error")}},
		&fixedMultiLoadPublisher{},
	)
	envelope, entryKey, err = s.ShipEntry(entry, authorPub, readerPub, kek, eek)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)
}

func TestShipReceive(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	getterBalancer := &fixedGetterBalancer{}
	putterBalancer := &fixedPutterBalancer{}
	authorKeys, readerKeys := keychain.New(3), keychain.New(3)
	authorKey, err := authorKeys.Sample()
	assert.Nil(t, err)
	authorPub := authorKey.PublicKeyBytes()
	readerKey, err := readerKeys.Sample()
	readerPub := readerKey.PublicKeyBytes()
	assert.Nil(t, err)
	kek, err := enc.NewKEK(authorKey.Key(), &readerKey.Key().PublicKey)
	assert.Nil(t, err)

	for _, nDocs := range []uint32{1, 2, 4, 8} {
		docSL1 := &fixedDocSLD{
			docs: make(map[string]*api.Document),
		}

		// make and store documents as if they had already been packed
		docs := make([]*api.Document, nDocs)
		docKeys := make([]id.ID, nDocs)
		for i := uint32(0); i < nDocs; i++ {
			entry := api.NewTestSinglePageEntry(rng)
			if i%2 == 1 {
				nPages := i * 2
				pageKeys := make([][]byte, nPages)
				for j := uint32(0); j < nPages; j++ {
					page := api.NewTestPage(rng)
					pageDoc, pageDocKey, err := api.GetPageDocument(page)
					assert.Nil(t, err)
					pageKeys[j] = pageDocKey.Bytes()
					err = docSL1.Store(pageDocKey, pageDoc)
					assert.Nil(t, err)
				}
				entry = api.NewTestMultiPageEntry(rng)
				entry.Contents.(*api.Entry_PageKeys).PageKeys.Keys = pageKeys
			}
			entry.AuthorPublicKey = authorPub
			docs[i] = &api.Document{
				Contents: &api.Document_Entry{
					Entry: entry,
				},
			}
			docKey, err := api.GetKey(docs[i])
			assert.Nil(t, err)
			docKeys[i] = docKey

			// load first SL with documents for publisher
			err = docSL1.Store(docKeys[i], docs[i])
			assert.Nil(t, err)
		}

		// ship all docs
		params := publish.NewDefaultParameters()
		pubAcq := &memPublisherAcquirer{
			docs: make(map[string]*api.Document),
		}
		mlP := publish.NewMultiLoadPublisher(
			publish.NewSingleLoadPublisher(pubAcq, docSL1),
			params,
		)
		s := NewShipper(putterBalancer, pubAcq, mlP).(*shipper)
		s.deletePages = false // so we can check them at the end
		eek := enc.NewPseudoRandomEEK(rng)
		envelopeKeys := make([]id.ID, nDocs)
		for i := uint32(0); i < nDocs; i++ {
			envelope, _, err := s.ShipEntry(docs[i], authorPub, readerPub, kek, eek)
			assert.Nil(t, err)
			envelopeKeys[i], err = api.GetKey(envelope)
			assert.Nil(t, err)
		}

		// receive all docs
		docSL2 := &fixedDocSLD{
			docs: make(map[string]*api.Document),
		}
		msA := publish.NewMultiStoreAcquirer(
			publish.NewSingleStoreAcquirer(pubAcq, docSL2),
			params,
		)
		r := NewReceiver(getterBalancer, readerKeys, pubAcq, msA, docSL2)
		for i := uint32(0); i < nDocs; i++ {
			entry, _, err := r.ReceiveEntry(envelopeKeys[i])
			assert.Equal(t, docs[i], entry)
			assert.Nil(t, err)
			entryKey, err := api.GetKey(entry)
			assert.Nil(t, err)
			assert.Equal(t, docKeys[i], entryKey)
			pageKeys, err := api.GetEntryPageKeys(entry)
			assert.Nil(t, err)
			for _, pageKey := range pageKeys {
				page1, err := docSL1.Load(pageKey)
				assert.Nil(t, err)
				page2, err := docSL2.Load(pageKey)
				assert.Nil(t, err)
				assert.Equal(t, page1, page2)
			}
		}
	}
}

type fixedMultiLoadPublisher struct {
	err     error
	deleted bool
}

func (f *fixedMultiLoadPublisher) Publish(
	docKeys []id.ID, authorPub []byte, cb client.PutterBalancer, delete bool,
) error {
	f.deleted = delete
	return f.err
}

type fixedPublisher struct {
	errs []error
}

func (f *fixedPublisher) Publish(doc *api.Document, authorPub []byte, lc api.Putter) (
	id.ID, error) {
	docID, err := api.GetKey(doc)
	if err != nil {
		return nil, err
	}
	if f.errs == nil {
		return docID, nil
	}
	nextErr := f.errs[0]
	f.errs = f.errs[1:]
	return docID, nextErr
}

type fixedGetterBalancer struct {
	client api.Getter
	err    error
}

func (f *fixedGetterBalancer) Next() (api.Getter, error) {
	return f.client, f.err
}

type fixedPutterBalancer struct {
	client api.Putter
	err    error
}

func (f *fixedPutterBalancer) Next() (api.Putter, error) {
	return f.client, f.err
}

type fixedDocSLD struct {
	docs        map[string]*api.Document
	mu          sync.Mutex
	loadError   error
	storeError  error
	deleteError error
}

func (f *fixedDocSLD) Load(key id.ID) (*api.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value := f.docs[key.String()]
	return value, f.loadError
}

func (f *fixedDocSLD) Store(key id.ID, value *api.Document) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.docs[key.String()] = value
	return f.storeError
}

func (f *fixedDocSLD) Delete(key id.ID) error {
	delete(f.docs, key.String())
	return f.deleteError
}

type memPublisherAcquirer struct {
	docs map[string]*api.Document
	mu   sync.Mutex
}

func (p *memPublisherAcquirer) Publish(doc *api.Document, authorPub []byte, lc api.Putter) (
	id.ID, error) {
	docKey, err := api.GetKey(doc)
	if err != nil {
		panic(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.docs[docKey.String()] = doc
	return docKey, nil
}

func (p *memPublisherAcquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (
	*api.Document, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.docs[docKey.String()], nil
}
