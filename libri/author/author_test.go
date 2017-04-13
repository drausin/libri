package author

import (
	"testing"
	"io/ioutil"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
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

func TestAuthor_Upload(t *testing.T) {
	// test
	// - mlPublisher gets called for pages
	// - publisher gets called for entry & envelope
	a := newTestAuthor()
	go func() { <- a.stop }() // dummy stop signal acceptor

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
