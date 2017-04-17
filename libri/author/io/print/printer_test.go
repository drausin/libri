package print

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultParameters(t *testing.T) {
	params := NewDefaultParameters()
	assert.NotNil(t, params)
}

func TestNewPrinter_ok(t *testing.T) {
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	assert.NotNil(t, params)
}

func TestNewPrinter_err(t *testing.T) {
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, 0)
	assert.Equal(t, ErrZeroParallelism, err)
	assert.Nil(t, params)
}

func TestPrinter_Print_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	nPages := 10
	keys, authorPub, _ := enc.NewPseudoRandomKeys(rng)
	fixedPageKeys, fixedPages := randPages(t, rng, nPages)
	readUncompressedN := nPages * int(page.MinSize)
	uncompressedSum := api.RandBytes(rng, api.HMAC256Length)
	readCiphertextN, ciphertextSum := readUncompressedN, api.RandBytes(rng, api.HMAC256Length)

	compressor := &fixedCompressor{
		readN:   readUncompressedN,
		readErr: nil,
		uncompressedMAC: &fixedMAC{
			messageSize: uint64(readUncompressedN),
			sum:         uncompressedSum,
		},
	}
	paginator := &fixedPaginator{
		readN:      int64(readCiphertextN),
		readErr:    nil,
		fixedPages: fixedPages,
		ciphertextMAC: &fixedMAC{
			messageSize: uint64(readCiphertextN),
			sum:         ciphertextSum,
		},
	}

	printer1 := NewPrinter(params, keys, authorPub, &fixedStorer{})
	printer1.(*printer).init = &fixedPrintInitializer{
		initCompressor: compressor,
		initPaginator:  paginator,
		initErr:        nil,
	}

	pageKeys, entryMetadata, err := printer1.Print(nil, "application/x-pdf")

	assert.Nil(t, err)
	assert.Equal(t, fixedPageKeys, pageKeys)
	actualCiphertextSize, _ := entryMetadata.GetCiphertextSize()
	assert.Equal(t, uint64(readCiphertextN), actualCiphertextSize)
	actualCiphertextSum, _ := entryMetadata.GetCiphertextMAC()
	assert.Equal(t, ciphertextSum, actualCiphertextSum)
}

func TestPrinter_Print_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	keys, authorPub, _ := enc.NewPseudoRandomKeys(rng)
	content, mediaType := bytes.NewReader(api.RandBytes(rng, 64)), "application/x-pdf"

	printer1 := NewPrinter(params, keys, authorPub, &fixedStorer{})
	printer1.(*printer).init = &fixedPrintInitializer{
		initCompressor: nil,
		initPaginator:  &fixedPaginator{},
		initErr:        errors.New("some Initialize error"),
	}

	// check that init error bubbles up
	pageKeys, entryMetadata, err := printer1.Print(content, mediaType)
	assert.NotNil(t, err)
	assert.Nil(t, pageKeys)
	assert.Nil(t, entryMetadata)

	storer2 := &fixedStorer{
		storeErr: errors.New("some Store error"),
	}
	printer2 := NewPrinter(params, keys, authorPub, storer2)
	printer2.(*printer).init = &fixedPrintInitializer{
		initCompressor: nil,
		initPaginator:  &fixedPaginator{},
		initErr:        nil,
	}

	// check that store error bubbles up
	pageKeys, entryMetadata, err = printer2.Print(content, mediaType)
	assert.NotNil(t, err)
	assert.Nil(t, pageKeys)
	assert.Nil(t, entryMetadata)

	paginator3 := &fixedPaginator{
		readN:   0,
		readErr: errors.New("some ReadFrom error"),
	}

	printer3 := NewPrinter(params, keys, authorPub, &fixedStorer{})
	printer3.(*printer).init = &fixedPrintInitializer{
		initCompressor: nil,
		initPaginator:  paginator3,
		initErr:        nil,
	}

	// check that paginator.ReadFrom error bubbles up
	pageKeys, entryMetadata, err = printer3.Print(content, mediaType)
	assert.NotNil(t, err)
	assert.Nil(t, pageKeys)
	assert.Nil(t, entryMetadata)

	compressor := &fixedCompressor{
		uncompressedMAC: &fixedMAC{
			messageSize: 0,
			sum:         []byte{},
		},
	}
	paginator := &fixedPaginator{
		ciphertextMAC: &fixedMAC{
			messageSize: 0,
			sum:         []byte{},
		},
	}
	printer4 := NewPrinter(params, keys, authorPub, &fixedStorer{})
	printer4.(*printer).init = &fixedPrintInitializer{
		initCompressor: compressor,
		initPaginator:  paginator,
		initErr:        nil,
	}

	// check that api.NewEntryMetadata error bubbles up
	pageKeys, entryMetadata, err = printer4.Print(content, mediaType)
	assert.NotNil(t, err)
	assert.Nil(t, pageKeys)
	assert.Nil(t, entryMetadata)
}

func TestPrintInitializerImpl_Initialize_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	keys, authorPub, _ := enc.NewPseudoRandomKeys(rng)
	content, mediaType := bytes.NewReader(api.RandBytes(rng, 64)), "application/x-pdf"
	pages := make(chan *api.Page)

	printInit := &printInitializerImpl{
		keys:      keys,
		authorPub: authorPub,
		params:    params,
	}
	compressor, paginator, err := printInit.Initialize(content, mediaType, pages)
	assert.Nil(t, err)
	assert.NotNil(t, compressor)
	assert.NotNil(t, paginator)
}

func TestPrintInitializerImpl_Initialize_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	keys, authorPub, _ := enc.NewPseudoRandomKeys(rng)
	content, mediaType := bytes.NewReader(api.RandBytes(rng, 64)), "application/x-pdf"
	pages := make(chan *api.Page)

	printInit1 := &printInitializerImpl{
		keys:      keys,
		authorPub: authorPub,
		params:    params,
	}

	// check that bad media type triggers error
	compressor, paginator, err := printInit1.Initialize(content, "application/", pages)
	assert.NotNil(t, err)
	assert.Nil(t, compressor)
	assert.Nil(t, paginator)

	printInit2 := &printInitializerImpl{
		keys:      keys,
		authorPub: authorPub,
		params: &Parameters{
			CompressionBufferSize: 0, // will trigger error when creating compressor
			PageSize:              page.MinSize,
			Parallelism:           DefaultParallelism,
		},
	}

	// check that error creating new compressor bubbles up
	compressor, paginator, err = printInit2.Initialize(content, mediaType, pages)
	assert.NotNil(t, err)
	assert.Nil(t, compressor)
	assert.Nil(t, paginator)

	keys3, _, _ := enc.NewPseudoRandomKeys(rng)
	keys3.AESKey = []byte{} // will trigger error when creating encrypter
	printInit3 := &printInitializerImpl{
		keys:      keys3,
		authorPub: authorPub,
		params:    params,
	}

	// check that error creating new encrypter triggers error
	compressor, paginator, err = printInit3.Initialize(content, mediaType, pages)
	assert.NotNil(t, err)
	assert.Nil(t, compressor)
	assert.Nil(t, paginator)

	keys4, _, _ := enc.NewPseudoRandomKeys(rng)
	keys4.HMACKey = []byte{} // will trigger error when creating paginator
	printInit4 := &printInitializerImpl{
		keys:      keys4,
		authorPub: authorPub,
		params:    params,
	}

	// check that error creating new encrypter triggers error
	compressor, paginator, err = printInit4.Initialize(content, mediaType, pages)
	assert.NotNil(t, err)
	assert.Nil(t, compressor)
	assert.Nil(t, paginator)
}

type fixedStorer struct {
	storeErr error
}

func (f *fixedStorer) Store(pages chan *api.Page) ([]cid.ID, error) {
	if f.storeErr != nil {
		return nil, f.storeErr
	}

	pageKeys := make([]cid.ID, 0)
	for chanPage := range pages {
		pageKey, err := api.GetKey(chanPage)
		if err != nil {
			return nil, err
		}
		pageKeys = append(pageKeys, pageKey)
	}
	return pageKeys, nil
}

type fixedPaginator struct {
	readN         int64
	readErr       error
	pages         chan *api.Page
	fixedPages    []*api.Page
	ciphertextMAC enc.MAC
}

func (f *fixedPaginator) ReadFrom(r io.Reader) (int64, error) {
	for _, fixedPage := range f.fixedPages {
		f.pages <- fixedPage
	}
	return f.readN, f.readErr
}

func (f *fixedPaginator) CiphertextMAC() enc.MAC {
	return f.ciphertextMAC
}

type fixedCompressor struct {
	readN           int
	readErr         error
	uncompressedMAC enc.MAC
}

func (f *fixedCompressor) Read(p []byte) (int, error) {
	return f.readN, f.readErr
}

func (f *fixedCompressor) UncompressedMAC() enc.MAC {
	return f.uncompressedMAC
}

type fixedPrintInitializer struct {
	initCompressor comp.Compressor
	initPaginator  *fixedPaginator
	initErr        error
}

func (f *fixedPrintInitializer) Initialize(
	content io.Reader, mediaType string, pages chan *api.Page,
) (comp.Compressor, page.Paginator, error) {

	f.initPaginator.pages = pages
	return f.initCompressor, f.initPaginator, f.initErr
}

type fixedMAC struct {
	messageSize uint64
	sum         []byte
}

func (f *fixedMAC) MessageSize() uint64 {
	return f.messageSize
}

func (f *fixedMAC) Sum(p []byte) []byte {
	return f.sum
}

func (f *fixedMAC) Reset() {}

func (f *fixedMAC) Write(p []byte) (int, error) {
	return int(f.messageSize), nil
}

func randPages(t *testing.T, rng *rand.Rand, n int) ([]cid.ID, []*api.Page) {
	pages := make([]*api.Page, n)
	pageKeys := make([]cid.ID, n)
	var err error
	for i := 0; i < n; i++ {
		pages[i] = api.NewTestPage(rng)
		pageKeys[i], err = api.GetKey(pages[i])
		assert.Nil(t, err)
	}
	return pageKeys, pages
}
