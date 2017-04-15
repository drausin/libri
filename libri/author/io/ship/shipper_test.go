package ship

import (
	"testing"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/id"
	"math/rand"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/stretchr/testify/assert"
	"github.com/pkg/errors"
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
	envelope, entryKey, err := s.Ship(entry, getPageKeys(entry), authorPub, readerPub)
	assert.Nil(t, err)
	assert.NotNil(t, envelope)
	assert.Equal(t, origEntryKey, entryKey)

	// test single-page ship
	pageKeys := []id.ID{id.NewPseudoRandom(rng)}
	envelope, entryKey, err = s.Ship(entry, pageKeys, authorPub, readerPub)
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

	// check inconsistent page keys trigger error
	pageKeys := []id.ID{id.NewPseudoRandom(rng), id.NewPseudoRandom(rng)}
	envelope, entryKey, err := s.Ship(entry, pageKeys, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	// check page publish error bubbles up
	pageKeys = getPageKeys(entry)
	envelope, entryKey, err = s.Ship(entry, pageKeys, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	s = NewShipper(
		&fixedClientBalancer{errors.New("some Next error")},
		&fixedPublisher{},
		&fixedMultiLoadPublisher{},
	)

	// check getting next librarian error bubbles up
	envelope, entryKey, err = s.Ship(entry, pageKeys, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	s = NewShipper(
		&fixedClientBalancer{},
		&fixedPublisher{[]error{errors.New("some Publish error")}},
		&fixedMultiLoadPublisher{},
	)

	// check entry publish error bubbles up
	envelope, entryKey, err = s.Ship(entry, pageKeys, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)

	s = NewShipper(
		&fixedClientBalancer{},
		&fixedPublisher{[]error{nil, errors.New("some Publish error")}},
		&fixedMultiLoadPublisher{},
	)

	// check envelope publish error bubbles up
	envelope, entryKey, err = s.Ship(entry, pageKeys, authorPub, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, envelope)
	assert.Nil(t, entryKey)
}

func getPageKeys(entry *api.Document) []id.ID {
	pageKeysBytes := entry.Contents.(*api.Document_Entry).Entry.
		Contents.(*api.Entry_PageKeys).PageKeys.Keys
	pageKeys := make([]id.ID, len(pageKeysBytes))
	for i, pageKeyBytes := range pageKeysBytes {
		pageKeys[i] = id.FromBytes(pageKeyBytes)
	}
	return pageKeys
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
