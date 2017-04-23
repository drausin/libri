package author

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"

	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/io/ship"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	"errors."
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

const (
	testKeychainAuth = "some secret passphrase"
)

func TestNewAuthor(t *testing.T) {
	a1 := newTestAuthor()
	go func() { <-a1.stop }() // dummy stop signal acceptor

	clientID1 := a1.clientID
	err := a1.Close()
	assert.Nil(t, err)

	a2, err := NewAuthor(a1.config, testKeychainAuth, clogging.NewDevInfoLogger())
	go func() { <-a2.stop }() // dummy stop signal acceptor

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
		envelope: &api.Document{
			Contents: &api.Document_Envelope{
				Envelope: api.NewTestEnvelope(rng),
			},
		},
		envelopeKey: expectedEntryKey,
	}
	go func() { <-a.stop }() // dummy stop signal acceptor

	// since everything is mocked, inputs don't really matter
	actualEnvelope, actualEnvelopeKey, err := a.Upload(nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, actualEnvelope)
	assert.Equal(t, expectedEntryKey, actualEnvelopeKey)

	err = a.CloseAndRemove()
	assert.Nil(t, err)
}

func TestAuthor_Upload_err(t *testing.T) {
	a := newTestAuthor()
	go func() { <-a.stop }() // dummy stop signal acceptor
	a.entryPacker = &fixedEntryPacker{err: errors.New("some Pack error")}
	a.shipper = &fixedShipper{}

	// check pack error bubbles up
	actualEnvelope, actualEnvelopeKey, err := a.Upload(nil, "")
	assert.NotNil(t, err)
	assert.Nil(t, actualEnvelope)
	assert.Nil(t, actualEnvelopeKey)

	a.entryPacker = &fixedEntryPacker{}
	a.shipper = &fixedShipper{err: errors.New("some Ship error")}

	// check pack error bubbles up
	actualEnvelope, actualEnvelopeKey, err = a.Upload(nil, "")
	assert.NotNil(t, err)
	assert.Nil(t, actualEnvelope)
	assert.Nil(t, actualEnvelopeKey)

	err = a.CloseAndRemove()
	assert.Nil(t, err)
}

func TestAuthor_Download_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	doc, docKey := api.NewTestDocument(rng)
	a := &Author{
		logger:        clogging.NewDevInfoLogger(),
		receiver:      &fixedReceiver{entry: doc},
		entryUnpacker: &fixedUnpacker{},
	}
	err := a.Download(nil, docKey)
	assert.Nil(t, err)
}

func TestAuthor_Download_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	doc, docKey := api.NewTestDocument(rng)

	// check Receive error bubbles up
	a1 := &Author{
		logger:        clogging.NewDevInfoLogger(),
		receiver:      &fixedReceiver{err: errors.New("some Receive error")},
		entryUnpacker: &fixedUnpacker{},
	}
	err := a1.Download(nil, docKey)
	assert.NotNil(t, err)

	// check Unpack error bubbles up
	a2 := &Author{
		logger:        clogging.NewDevInfoLogger(),
		receiver:      &fixedReceiver{entry: doc},
		entryUnpacker: &fixedUnpacker{err: errors.New("some Unpack error")},
	}
	err = a2.Download(nil, docKey)
	assert.NotNil(t, err)
}

func TestAuthor_UploadDownload(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	a := newTestAuthor()
	go func() { <-a.stop }() // dummy stop signal acceptor
	a.librarians = &fixedClientBalancer{}

	// just mock interaction with libri network
	pubAcq := &memPublisherAcquirer{
		docs: make(map[string]*api.Document),
	}

	// but need to re-init shipper & reciever via publishers/acquirers
	slPublisher := publish.NewSingleLoadPublisher(pubAcq, a.documentSL)
	ssAcquirer := publish.NewSingleStoreAcquirer(pubAcq, a.documentSL)
	mlPublisher := publish.NewMultiLoadPublisher(slPublisher, a.config.Publish)
	msAcquirer := publish.NewMultiStoreAcquirer(ssAcquirer, a.config.Publish)
	a.shipper = ship.NewShipper(a.librarians, pubAcq, mlPublisher)
	a.receiver = ship.NewReceiver(a.librarians, a.selfReaderKeys, pubAcq, msAcquirer,
		a.documentSL)

	page.MinSize = 64 // just for testing
	pageSizes := []uint32{128, 256, 512}
	uncompressedSizes := []int{128, 192, 256, 384, 512, 768, 1024, 2048}
	mediaTypes := []string{"application/x-pdf", "application/x-gzip"}
	cases := caseCrossProduct(pageSizes, uncompressedSizes, mediaTypes)

	for _, c := range cases {
		a.config.Print.PageSize = c.pageSize

		content1 := common.NewCompressableBytes(rng, c.uncompressedSize)
		content1Bytes := content1.Bytes()

		envelope, envelopeKey, err := a.Upload(content1, c.mediaType)
		assert.Nil(t, err)
		assert.NotNil(t, envelope)
		assert.NotNil(t, envelopeKey)

		content2 := new(bytes.Buffer)
		err = a.Download(content2, envelopeKey)
		assert.Nil(t, err)

		// check content1 == content1 --> Upload --> Download
		assert.Equal(t, content1Bytes, content2.Bytes())
	}

	err := a.CloseAndRemove()
	assert.Nil(t, err)
}

type fixedPublisher struct {
	doc        *api.Document
	lc         api.Putter
	publishID  id.ID
	publishErr error
}

func (f *fixedPublisher) Publish(doc *api.Document, lc api.Putter) (id.ID, error) {
	f.doc, f.lc = doc, lc
	return f.publishID, f.publishErr
}

type fixedMLPublisher struct {
	docKeys    []id.ID
	cb         api.ClientBalancer
	publishErr error
}

func (f *fixedMLPublisher) Publish(docKeys []id.ID, cb api.ClientBalancer) error {
	f.docKeys, f.cb = docKeys, cb
	return f.publishErr
}

type fixedEntryPacker struct {
	entry *api.Document
	err   error
}

func (f *fixedEntryPacker) Pack(
	content io.Reader, mediaType string, keys *enc.Keys, authorPub []byte,
) (*api.Document, error) {
	return f.entry, f.err
}

type fixedShipper struct {
	envelope    *api.Document
	envelopeKey id.ID
	err         error
}

func (f *fixedShipper) Ship(entry *api.Document, authorPub []byte, readerPub []byte) (
	*api.Document, id.ID, error) {
	return f.envelope, f.envelopeKey, f.err
}

type fixedReceiver struct {
	entry *api.Document
	keys  *enc.Keys
	err   error
}

func (f *fixedReceiver) Receive(envelopeKey id.ID) (*api.Document, *enc.Keys, error) {
	return f.entry, f.keys, f.err
}

type fixedUnpacker struct {
	err error
}

func (f *fixedUnpacker) Unpack(content io.Writer, entry *api.Document, keys *enc.Keys) error {
	return f.err
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

type fixedClientBalancer struct {
	client api.LibrarianClient
	err    error
}

func (f *fixedClientBalancer) Next() (api.LibrarianClient, error) {
	return f.client, f.err
}

func (f *fixedClientBalancer) CloseAll() error {
	return f.err
}

type packTestCase struct {
	pageSize          uint32
	uncompressedSize  int
	mediaType         string
	packParallelism   uint32
	unpackParallelism uint32
}

func (p packTestCase) String() string {
	return fmt.Sprintf("pageSize: %d, uncompressedSize: %d, mediaType: %s", p.pageSize,
		p.uncompressedSize, p.mediaType)
}

type upDownTestCase struct {
	pageSize         uint32
	uncompressedSize int
	mediaType        string
}

func caseCrossProduct(
	pageSizes []uint32,
	uncompressedSizes []int,
	mediaTypes []string,
) []*upDownTestCase {
	cases := make([]*upDownTestCase, 0)
	for _, pageSize := range pageSizes {
		for _, uncompressedSize := range uncompressedSizes {
			for _, mediaType := range mediaTypes {
				cases = append(cases, &upDownTestCase{
					pageSize:         pageSize,
					uncompressedSize: uncompressedSize,
					mediaType:        mediaType,
				})
			}
		}
	}
	return cases
}
func newTestAuthor() *Author {
	config := newTestConfig()
	logger := clogging.NewDevLogger(zapcore.DebugLevel)

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
