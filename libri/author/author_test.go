package author

import (
	"testing"
	"io/ioutil"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"io"
	"github.com/drausin/libri/libri/author/io/enc"
	"math/rand"
	"github.com/pkg/errors"
)

const (
	testKeychainAuth = "some secret passphrase"
)

func TestNewAuthor(t *testing.T) {
	a1 := newTestAuthor()
	go func() { <- a1.stop }() // dummy stop signal acceptor

	clientID1 := a1.clientID
	err := a1.Close()
	assert.Nil(t, err)

	a2, err := NewAuthor(a1.config, testKeychainAuth, clogging.NewDevInfoLogger())
	go func() { <- a2.stop }() // dummy stop signal acceptor

	assert.Nil(t, err)
	assert.Equal(t, clientID1, a2.clientID)
	err = a2.CloseAndRemove()
	assert.Nil(t, err)
}

func TestAuthor_Upload_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	a := newTestAuthor()
	a.entryPacker = &fixedEntryPacker{}
	expectedEntryKey := id.NewPseudoRandom(rng)
	a.shipper = &fixedShipper{
		entryKey: expectedEntryKey,
	}
	go func() { <- a.stop }() // dummy stop signal acceptor

	// since everything is mocked, inputs don't really matter
	actualEntryKey, err := a.Upload(nil, "")
	assert.Nil(t, err)
	assert.Equal(t, expectedEntryKey, actualEntryKey)

	err = a.CloseAndRemove()
	assert.Nil(t, err)
}

func TestAuthor_Upload_err(t *testing.T) {
	a := newTestAuthor()
	go func() { <- a.stop }() // dummy stop signal acceptor
	a.entryPacker = &fixedEntryPacker{err: errors.New("some Pack error")}
	a.shipper = &fixedShipper{}

	// check pack error bubbles up
	actualEntryKey, err := a.Upload(nil, "")
	assert.NotNil(t, err)
	assert.Nil(t, actualEntryKey)

	a.entryPacker = &fixedEntryPacker{}
	a.shipper = &fixedShipper{err: errors.New("some Ship error")}

	// check pack error bubbles up
	actualEntryKey, err = a.Upload(nil, "")
	assert.NotNil(t, err)
	assert.Nil(t, actualEntryKey)

	err = a.CloseAndRemove()
	assert.Nil(t, err)
}

type fixedPublisher struct {
	doc *api.Document
	lc api.Putter
	publishID id.ID
	publishErr error
}

func (f *fixedPublisher) Publish(doc *api.Document, lc api.Putter) (id.ID, error) {
	f.doc, f.lc = doc, lc
	return f.publishID, f.publishErr
}

type fixedMLPublisher struct {
	docKeys []id.ID
	cb api.ClientBalancer
	publishErr error
}

func (f *fixedMLPublisher) Publish(docKeys []id.ID, cb api.ClientBalancer) error {
	f.docKeys, f.cb = docKeys, cb
	return f.publishErr
}

type fixedEntryPacker struct {
	entry    *api.Document
	pageKeys []id.ID
	err      error
}

func (f *fixedEntryPacker) Pack(
	content io.Reader, mediaType string, keys *enc.Keys, authorPub []byte,
) (*api.Document, []id.ID, error) {
	return f.entry, f.pageKeys, f.err
}

type fixedShipper struct {
	envelope *api.Document
	entryKey id.ID
	err error
}

func (f *fixedShipper) Ship(
	entry *api.Document, pageKeys []id.ID, authorPub []byte, readerPub []byte,
) (*api.Document, id.ID, error) {
	return f.envelope, f.entryKey, f.err
}

func newTestAuthor() *Author {
	config := newTestConfig()
	logger := clogging.NewDevInfoLogger()

	// create keychains
	err := createKeychains(logger, config.KeychainDir, testKeychainAuth,
		veryLightScryptN, veryLightScryptP)
	if err != nil {
		panic(err)
	}

	author, err := NewAuthor(config, testKeychainAuth, logger)
	if err != nil {
		panic(err)
	}
	return author
}

func newTestConfig() *Config {
	config := NewDefaultConfig()
	dir, err := ioutil.TempDir("", "author-test-data-dir")
	if err != nil {
		panic(err)
	}

	// set data dir and resets DB and Keychain dirs to use it
	config.WithDataDir(dir).
		WithDefaultDBDir().
		WithDefaultKeychainDir()

	return config
}
