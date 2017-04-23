package ship

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/id"
	"math/rand"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/stretchr/testify/assert"
	"github.com/pkg/errors"
	"github.com/drausin/libri/libri/author/io/publish"
	"sync"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
)

func TestShipper_Ship_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	_, authorPub, readerPub := enc.NewPseudoRandomKeys(rng)
	s := NewShipper(
		&fixedClientBalancer{},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{},
	)
	entry := &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestMultiPageEntry(rng),
		},
	}
	origEntryKey, err := api.GetKey(entry)
	assert.Nil(t, err)

	// test multi-page ship
	envelope, entryKey, err := s.Ship(entry, authorPub, readerPub)
	assert.Nil(t, err)
	assert.NotNil(t, envelope)
	assert.Equal(t, origEntryKey, entryKey)

	// test single-page ship
	entry = &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestSinglePageEntry(rng),
		},
	}
	origEntryKey, err = api.GetKey(entry)
	assert.Nil(t, err)
	envelope, entryKey, err = s.Ship(entry, authorPub, readerPub)
	assert.Nil(t, err)
	assert.NotNil(t, envelope)
	assert.Equal(t, origEntryKey, entryKey)
}

func TestShipper_Ship_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	_, authorPub, readerPub := enc.NewPseudoRandomKeys(rng)
	entry := &api.Document{
		Contents: &api.Document_Entry{
			Entry: api.NewTestMultiPageEntry(rng),
		},
	}

	s := NewShipper(
		&fixedClientBalancer{},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{errors.New("some Publish error")},
	)

	// check page publish error bubbles up
	envelope, entryKey, err := s.Ship(entry, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	s = NewShipper(
		&fixedClientBalancer{errors.New("some Next error")},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{},
	)

	// check getting next librarian error bubbles up
	envelope, entryKey, err = s.Ship(entry, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	s = NewShipper(
		&fixedClientBalancer{},
		&fixedPublisher{[]error{errors.New("some Publish error")}},
		&fixedMultiLoadPublisher{},
	)

	// check entry publish error bubbles up
	envelope, entryKey, err = s.Ship(entry, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	s = NewShipper(
		&fixedClientBalancer{},
		&fixedPublisher{[]error{nil, errors.New("some Publish error")}},
		&fixedMultiLoadPublisher{},
	)

	// check envelope publish error bubbles up
	envelope, entryKey, err = s.Ship(entry, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)
}

func TestShipReceive(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	cb := &fixedClientBalancer{}
	authorKeys, readerKeys := keychain.New(3), keychain.New(3)
	authorKey, err := authorKeys.Sample()
	assert.Nil(t, err)
	authorPub := ecid.ToPublicKeyBytes(authorKey)
	readerKey, err := readerKeys.Sample()
	readerPub := ecid.ToPublicKeyBytes(readerKey)
	assert.Nil(t, err)
	for _, nDocs := range []uint32{1, 2, 4, 8} {
		docSL1 := &memDocStorerLoader{
			docs: make(map[string]*api.Document),
		}

		// make and store documents as if they had already been packed
		docs := make([]*api.Document, nDocs)
		docKeys := make([]id.ID, nDocs)
		for i := uint32(0); i < nDocs; i++ {
			entry := api.NewTestSinglePageEntry(rng)
			if i % 2 == 1 {
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
			docSL1.Store(docKeys[i], docs[i])
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
		s := NewShipper(cb, pubAcq, mlP)
		envelopeKeys := make([]id.ID, nDocs)
		for i := uint32(0); i < nDocs; i++ {
			envelope, _, err := s.Ship(docs[i], authorPub, readerPub)
			assert.Nil(t, err)
			envelopeKeys[i], err = api.GetKey(envelope)
		}

		// receive all docs
		docSL2 := &memDocStorerLoader{
			docs: make(map[string]*api.Document),
		}
		msA := publish.NewMultiStoreAcquirer(
			publish.NewSingleStoreAcquirer(pubAcq, docSL2),
			params,
		)
		r := NewReceiver(cb, readerKeys, pubAcq, msA, docSL2)
		for i := uint32(0); i < nDocs; i++ {
			entry, _, err := r.Receive(envelopeKeys[i])
			assert.Equal(t, docs[i], entry)
			assert.Nil(t, err)
			entryKey, err := api.GetKey(entry)
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
	err error
}

func (f *fixedMultiLoadPublisher) Publish(
	docKeys []id.ID, authorPub []byte, cb api.ClientBalancer,
) error {
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

type fixedClientBalancer struct {
	err error
}

func (f *fixedClientBalancer) Next() (api.LibrarianClient, error) {
	return nil, f.err
}

func (f *fixedClientBalancer) CloseAll() error {
	return nil
}

type memDocStorerLoader struct {
	docs map[string]*api.Document
	mu sync.Mutex
}

func (m *memDocStorerLoader) Load(key id.ID) (*api.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, _ := m.docs[key.String()]
	return value, nil
}

func (m *memDocStorerLoader) Store(key id.ID, value *api.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[key.String()] = value
	return nil
}

type memPublisherAcquirer struct {
	docs map[string]*api.Document
	mu sync.Mutex
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
