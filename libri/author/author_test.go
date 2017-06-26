package author

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"
	"errors"
	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/author/io/ship"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/api"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"net"
	"google.golang.org/grpc"
	"golang.org/x/net/context"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"crypto/ecdsa"
	"crypto/elliptic"
)

const (
	testKeychainAuth = "some secret passphrase"
)

func TestNewAuthor(t *testing.T) {
	// return empty map of health clients
	orig := getLibrarianHealthClients
	getLibrarianHealthClients = func(librarianAddrs []*net.TCPAddr) (
		map[string]healthpb.HealthClient, error) {
		return make(map[string]healthpb.HealthClient), nil
	}
	defer func() { getLibrarianHealthClients = orig }()

	a1 := newTestAuthor()

	clientID1 := a1.clientID
	err := a1.Close()
	assert.Nil(t, err)

	a2, err := NewAuthor(
		a1.config,
		a1.authorKeys,
		a1.selfReaderKeys.(keychain.GetterSampler),
		clogging.NewDevInfoLogger(),
	)

	assert.Nil(t, err)
	assert.Equal(t, clientID1, a2.clientID)
	err = a2.CloseAndRemove()
	assert.Nil(t, err)
}

func TestAuthor_Healthcheck_ok(t *testing.T) {
	// return fixed map of health clients
	orig := getLibrarianHealthClients
	getLibrarianHealthClients = func(librarianAddrs []*net.TCPAddr) (
		map[string]healthpb.HealthClient, error) {
		return map[string]healthpb.HealthClient{
			"peerAddr1": &fixedHealthClient{
				response: &healthpb.HealthCheckResponse{
					Status: healthpb.HealthCheckResponse_SERVING,
				},
			},
			"peerAddr2": &fixedHealthClient{
				response: &healthpb.HealthCheckResponse{
					Status: healthpb.HealthCheckResponse_NOT_SERVING,
				},
			},
		}, nil
	}
	defer func() { getLibrarianHealthClients = orig }()

	a := newTestAuthor()

	allHealthy, healthStatus := a.Healthcheck()
	assert.False(t, allHealthy)
	assert.Equal(t, 2, len(healthStatus))
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, healthStatus["peerAddr1"])
	assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, healthStatus["peerAddr2"])
}

func TestAuthor_Healthcheck_err(t *testing.T) {
	// return fixed map of health clients
	orig := getLibrarianHealthClients
	getLibrarianHealthClients = func(librarianAddrs []*net.TCPAddr) (
		map[string]healthpb.HealthClient, error) {
		return map[string]healthpb.HealthClient{
			"peerAddr1": &fixedHealthClient{
				err: errors.New("some Check error"),
			},
		}, nil
	}
	defer func() { getLibrarianHealthClients = orig }()

	a := newTestAuthor()

	allHealthy, healthStatus := a.Healthcheck()
	assert.False(t, allHealthy)
	assert.Equal(t, 1, len(healthStatus))
	assert.Equal(t, healthpb.HealthCheckResponse_UNKNOWN, healthStatus["peerAddr1"])
}

func TestAuthor_Upload_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	a := newTestAuthor()
	metadata, err := api.NewEntryMetadata(
		"application/x-pdf",
		1,
		api.RandBytes(rng, 32),
		2,
		api.RandBytes(rng, 32),
	)
	assert.Nil(t, err)
	a.entryPacker = &fixedEntryPacker{
		metadata: metadata,
	}
	expectedEnvKey := id.NewPseudoRandom(rng)
	a.shipper = &fixedShipper{
		envelope: &api.Document{
			Contents: &api.Document_Envelope{
				Envelope: api.NewTestEnvelope(rng),
			},
		},
		envelopeKey: expectedEnvKey,
	}

	// since everything is mocked, inputs don't really matter
	actualEnvelope, actualEnvelopeKey, err := a.Upload(nil, "")
	assert.Nil(t, err)
	assert.NotNil(t, actualEnvelope)
	assert.Equal(t, expectedEnvKey, actualEnvelopeKey)

	err = a.CloseAndRemove()
	assert.Nil(t, err)
}

func TestAuthor_Upload_err(t *testing.T) {
	a := newTestAuthor()
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
	metadata, err := api.NewEntryMetadata(
		"application/x-pdf",
		1,
		api.RandBytes(rng, 32),
		2,
		api.RandBytes(rng, 32),
	)
	assert.Nil(t, err)
	a := &Author{
		logger:        clogging.NewDevInfoLogger(),
		receiver:      &fixedReceiver{entry: doc},
		entryUnpacker: &fixedUnpacker{metadata: metadata},
	}
	err = a.Download(nil, docKey)
	assert.Nil(t, err)
}

func TestAuthor_Download_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	doc, docKey := api.NewTestDocument(rng)

	// check Receive error bubbles up
	a1 := &Author{
		logger:        clogging.NewDevInfoLogger(),
		receiver:      &fixedReceiver{receiveEntryErr: errors.New("some Receive error")},
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
	a.librarians = &fixedClientBalancer{}

	// just mock interaction with libri network
	pubAcq := &memPublisherAcquirer{
		docs: make(map[string]*api.Document),
	}

	// but need to re-init shipper & reciever via publishers/acquirers
	slPublisher := publish.NewSingleLoadPublisher(pubAcq, a.documentSLD)
	ssAcquirer := publish.NewSingleStoreAcquirer(pubAcq, a.documentSLD)
	mlPublisher := publish.NewMultiLoadPublisher(slPublisher, a.config.Publish)
	msAcquirer := publish.NewMultiStoreAcquirer(ssAcquirer, a.config.Publish)
	a.shipper = ship.NewShipper(a.librarians, pubAcq, mlPublisher)
	a.receiver = ship.NewReceiver(a.librarians, a.selfReaderKeys, pubAcq, msAcquirer,
		a.documentSLD)

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

func TestAuthor_Share_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	a := newTestAuthor()
	defer func() {
		err := a.CloseAndRemove()
		assert.Nil(t, err)
	}()
	a.receiver = &fixedReceiver{
		envelope: api.NewTestEnvelope(rng),
		eek: enc.NewPseudoRandomEEK(rng),
	}
	expectedSharedEnvKey := id.NewPseudoRandom(rng)
	a.shipper = &fixedShipper{
		envelope: &api.Document{
			Contents: &api.Document_Envelope{
				Envelope: api.NewTestEnvelope(rng),
			},
		},
		envelopeKey: expectedSharedEnvKey,
	}

	// since everything is mocked, inputs don't really matter
	origEnvKey := id.NewPseudoRandom(rng)
	readerPub := &ecid.NewPseudoRandom(rng).Key().PublicKey
	actualSharedEnv, actualSharedEnvKey, err := a.Share(origEnvKey, readerPub)
	assert.Nil(t, err)
	assert.NotNil(t, actualSharedEnv)
	assert.Equal(t, expectedSharedEnvKey, actualSharedEnvKey)
}

func TestAuthor_Share_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	origEnvKey := id.NewPseudoRandom(rng)
	readerPub := &ecid.NewPseudoRandom(rng).Key().PublicKey

	// check ReceiveEnvelope error bubbles up
	a1 := &Author{
		receiver: &fixedReceiver{
			receiveEnvelopeErr: errors.New("some ReceiveEnvelope error"),
		},
	}
	env, envID, err := a1.Share(origEnvKey, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, env)
	assert.Nil(t, envID)

	// check GetEEK error bubbles up
	a2 := &Author{
		receiver: &fixedReceiver{
			getErrkErr: errors.New("some GetEEK error"),
		},
	}
	env, envID, err = a2.Share(origEnvKey, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, env)
	assert.Nil(t, envID)

	// check Share error bubbles up
	a3 := &Author{
		receiver: &fixedReceiver{},
		authorKeys: &fixedKeychain{
			sampleErr: errors.New("some Sample error"),
		},
	}
	env, envID, err = a3.Share(origEnvKey, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, env)
	assert.Nil(t, envID)

	// check NewKEK error bubbles up
	badCurvePK, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	a4 := &Author{
		receiver: &fixedReceiver{},
		authorKeys: &fixedKeychain{
			sampleID: ecid.FromPrivateKey(badCurvePK),
		},
	}
	env, envID, err = a4.Share(origEnvKey, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, env)
	assert.Nil(t, envID)

	a5 := &Author{
		receiver: &fixedReceiver{
			envelope: api.NewTestEnvelope(rng),
		},
		authorKeys: keychain.New(1),
		shipper: &fixedShipper{
			err: errors.New("some ShipEnvelope error"),
		},
	}
	env, envID, err = a5.Share(origEnvKey, readerPub)
	assert.NotNil(t, err)
	assert.Nil(t, env)
	assert.Nil(t, envID)
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
	entry    *api.Document
	metadata *api.Metadata
	err      error
}

func (f *fixedEntryPacker) Pack(
	content io.Reader, mediaType string, keys *enc.EEK, authorPub []byte,
) (*api.Document, *api.Metadata, error) {
	return f.entry, f.metadata, f.err
}

type fixedShipper struct {
	envelope    *api.Document
	envelopeKey id.ID
	err         error
}

func (f *fixedShipper) ShipEntry(
	entry *api.Document, authorPub []byte, readerPub []byte, kek *enc.KEK, eek *enc.EEK,
) (*api.Document, id.ID, error) {
	return f.envelope, f.envelopeKey, f.err
}

func (f *fixedShipper) ShipEnvelope(
	kek *enc.KEK, eek *enc.EEK, entryKey id.ID, authorPub, readerPub []byte,
) (*api.Document, id.ID, error) {
	return f.envelope, f.envelopeKey, f.err
}

type fixedReceiver struct {
	entry              *api.Document
	keys               *enc.EEK
	receiveEntryErr    error
	envelope           *api.Envelope
	receiveEnvelopeErr error
	eek                *enc.EEK
	getErrkErr         error
}

func (f *fixedReceiver) ReceiveEntry(envelopeKey id.ID) (*api.Document, *enc.EEK, error) {
	return f.entry, f.keys, f.receiveEntryErr
}

func (f *fixedReceiver) ReceiveEnvelope(envelopeKey id.ID) (*api.Envelope, error) {
	return f.envelope, f.receiveEnvelopeErr
}

func (f *fixedReceiver) GetEEK(envelope *api.Envelope) (*enc.EEK, error) {
	return f.eek, f.getErrkErr
}

type fixedUnpacker struct {
	metadata *api.Metadata
	err      error
}

func (f *fixedUnpacker) Unpack(content io.Writer, entry *api.Document, keys *enc.EEK) (
	*api.Metadata, error) {
	return f.metadata, f.err
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

type fixedHealthClient struct {
	response *healthpb.HealthCheckResponse
	err      error
}

func (f *fixedHealthClient) Check(
	ctx context.Context, in *healthpb.HealthCheckRequest, opts ...grpc.CallOption,
) (*healthpb.HealthCheckResponse, error) {
	return f.response, f.err
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
	err := CreateKeychains(logger, config.KeychainDir, testKeychainAuth,
		veryLightScryptN, veryLightScryptP)
	if err != nil {
		panic(err)
	}

	authorKeys, selfReaderKeys := keychain.New(nInitialKeys), keychain.New(nInitialKeys)
	author, err := NewAuthor(config, authorKeys, selfReaderKeys, logger)
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
