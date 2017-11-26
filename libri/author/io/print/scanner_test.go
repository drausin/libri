package print

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestScanner_Scan_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	nPages := 10
	pageKeys, _ := randPages(t, rng, nPages)
	keys := enc.NewPseudoRandomEEK(rng)
	writeUncompressedN := uint64(nPages * int(page.MinSize))
	uncompressedSum := api.RandBytes(rng, api.HMAC256Length)
	writeCiphertextN := writeUncompressedN
	ciphertextSum := api.RandBytes(rng, api.HMAC256Length)

	decompressor := &fixedDecompressor{
		writeN:   int(writeUncompressedN),
		writeErr: nil,
		uncompressedMAC: &fixedMAC{
			messageSize: writeUncompressedN,
			sum:         uncompressedSum,
		},
	}
	unpaginator := &fixedUnpaginator{
		writeN:   int64(writeCiphertextN),
		writeErr: nil,
		ciphertextMAC: &fixedMAC{
			messageSize: writeCiphertextN,
			sum:         ciphertextSum,
		},
	}

	scanner1 := NewScanner(params, &fixedLoader{})
	scanner1.(*scanner).init = &fixedScanInitializer{
		initDecompressor: decompressor,
		initUnpaginator:  unpaginator,
		initErr:          nil,
	}
	entryMetadata := &api.EntryMetadata{
		MediaType:        "application/x-pdf",
		CiphertextSize:   writeCiphertextN,
		CiphertextMac:    ciphertextSum,
		UncompressedSize: writeUncompressedN,
		UncompressedMac:  uncompressedSum,
	}

	err = scanner1.Scan(nil, pageKeys, keys, entryMetadata)
	assert.Nil(t, err)
	assert.Equal(t, pageKeys, pageKeys)
	assert.Equal(t, uint64(writeCiphertextN), entryMetadata.CiphertextSize)
	assert.Equal(t, ciphertextSum, entryMetadata.CiphertextMac)
}

func TestScanner_Scan_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	nPages := 10
	keys := enc.NewPseudoRandomEEK(rng)
	pageKeys, _ := randPages(t, rng, nPages)
	content, mediaType := new(bytes.Buffer), "application/x-pdf"
	entryMetadata := &api.EntryMetadata{
		MediaType:        mediaType,
		CiphertextSize:   1,
		CiphertextMac:    api.RandBytes(rng, api.HMAC256Length),
		UncompressedSize: 3,
		UncompressedMac:  api.RandBytes(rng, api.HMAC256Length),
	}
	assert.Nil(t, err)

	// check that invalid metadata triggers error
	scanner1 := NewScanner(params, &fixedLoader{})
	md1 := &api.EntryMetadata{} // empty, so missing all fields
	err = scanner1.Scan(content, pageKeys, keys, md1)
	assert.NotNil(t, err)

	// check that init error bubbles up
	scanner2 := NewScanner(params, &fixedLoader{})
	scanner2.(*scanner).init = &fixedScanInitializer{
		initDecompressor: nil,
		initUnpaginator:  &fixedUnpaginator{},
		initErr:          errors.New("some Initialize error"),
	}
	err = scanner2.Scan(content, pageKeys, keys, entryMetadata)
	assert.NotNil(t, err)

	// check that load error bubbles up
	loader3 := &fixedLoader{
		loadErr: errors.New("some Load error"),
	}
	scanner3 := NewScanner(params, loader3)
	scanner3.(*scanner).init = &fixedScanInitializer{
		initDecompressor: nil,
		initUnpaginator:  &fixedUnpaginator{},
		initErr:          nil,
	}
	err = scanner3.Scan(content, pageKeys, keys, entryMetadata)
	assert.NotNil(t, err)

	// check that unpaginator.WriteTo error bubbles up
	unpaginator4 := &fixedUnpaginator{
		writeN:   0,
		writeErr: errors.New("some WriteTo error"),
	}
	scanner4 := NewScanner(params, &fixedLoader{})
	scanner4.(*scanner).init = &fixedScanInitializer{
		initDecompressor: nil,
		initUnpaginator:  unpaginator4,
		initErr:          nil,
	}
	err = scanner4.Scan(content, pageKeys, keys, entryMetadata)
	assert.NotNil(t, err)

	// check that MAC check error bubbles up
	decompressor := &fixedDecompressor{
		uncompressedMAC: &fixedMAC{
			messageSize: 0,
			sum:         []byte{},
		},
	}
	unpaginator := &fixedUnpaginator{
		ciphertextMAC: &fixedMAC{
			messageSize: 0,
			sum:         []byte{},
		},
	}
	scanner5 := NewScanner(params, &fixedLoader{})
	scanner5.(*scanner).init = &fixedScanInitializer{
		initDecompressor: decompressor,
		initUnpaginator:  unpaginator,
		initErr:          nil,
	}
	err = scanner5.Scan(content, pageKeys, keys, entryMetadata)
	assert.NotNil(t, err)
}

func TestScanInitializerImpl_Initialize_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	keys := enc.NewPseudoRandomEEK(rng)
	content, codec := new(bytes.Buffer), api.CompressionCodec_GZIP
	pages := make(chan *api.Page)

	scanInit := &scanInitializerImpl{params: params}
	decompressor, unpaginator, err := scanInit.Initialize(content, codec, keys, pages)
	assert.Nil(t, err)
	assert.NotNil(t, decompressor)
	assert.NotNil(t, unpaginator)
}

func TestScanInitializerImpl_Initialize_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params, err := NewParameters(comp.MinBufferSize, page.MinSize, DefaultParallelism)
	assert.Nil(t, err)
	keys := enc.NewPseudoRandomEEK(rng)
	content, codec := new(bytes.Buffer), api.CompressionCodec_GZIP
	pages := make(chan *api.Page)

	scanInit1 := &scanInitializerImpl{
		params: &Parameters{
			CompressionBufferSize: 0, // will trigger error when creating decompressor
			PageSize:              page.MinSize,
			Parallelism:           DefaultParallelism,
		},
	}

	// check that error creating new decompressor bubbles up
	decompressor, unpaginator, err := scanInit1.Initialize(content, codec, keys, pages)
	assert.NotNil(t, err)
	assert.Nil(t, decompressor)
	assert.Nil(t, unpaginator)

	keys2 := enc.NewPseudoRandomEEK(rng)
	keys2.AESKey = []byte{} // will trigger error when creating decrypter
	scanInit2 := &scanInitializerImpl{
		params: params,
	}

	// check that error creating new decrypter triggers error
	decompressor, unpaginator, err = scanInit2.Initialize(content, codec, keys2, pages)
	assert.NotNil(t, err)
	assert.Nil(t, decompressor)
	assert.Nil(t, unpaginator)

	keys3 := enc.NewPseudoRandomEEK(rng)
	keys3.HMACKey = []byte{} // will trigger error when creating unpaginator
	scanInit3 := &scanInitializerImpl{
		params: params,
	}

	// check that error creating new decrypter triggers error
	decompressor, unpaginator, err = scanInit3.Initialize(content, codec, keys3, pages)
	assert.NotNil(t, err)
	assert.Nil(t, decompressor)
	assert.Nil(t, unpaginator)
}

type fixedLoader struct {
	loadErr error
	pages   map[string]*api.Page
}

func (f *fixedLoader) Load(keys []id.ID, pages chan *api.Page, abort chan struct{}) error {
	if f.loadErr != nil {
		return f.loadErr
	}
	for _, key := range keys {
		p := f.pages[key.String()]
		select {
		case <-pages:
			return nil
		default:
			pages <- p
		}
	}
	return nil
}

type fixedUnpaginator struct {
	writeN        int64
	writeErr      error
	pages         chan *api.Page
	ciphertextMAC enc.MAC
}

func (f *fixedUnpaginator) WriteTo(w comp.CloseWriter) (int64, error) {
	if f.writeErr != nil {
		return f.writeN, f.writeErr
	}
	for range f.pages {
		// just consume pages and do nothing
	}
	return f.writeN, f.writeErr
}

func (f *fixedUnpaginator) CiphertextMAC() enc.MAC {
	return f.ciphertextMAC
}

type fixedDecompressor struct {
	writeN          int
	writeErr        error
	uncompressedMAC enc.MAC
}

func (f *fixedDecompressor) Write(p []byte) (int, error) {
	return f.writeN, f.writeErr
}

func (f *fixedDecompressor) UncompressedMAC() enc.MAC {
	return f.uncompressedMAC
}

func (f *fixedDecompressor) Close() error {
	return nil
}

type fixedScanInitializer struct {
	initDecompressor comp.Decompressor
	initUnpaginator  *fixedUnpaginator
	initErr          error
}

func (f *fixedScanInitializer) Initialize(
	content io.Writer, codec api.CompressionCodec, keys *enc.EEK, pages chan *api.Page,
) (comp.Decompressor, page.Unpaginator, error) {

	f.initUnpaginator.pages = pages
	return f.initDecompressor, f.initUnpaginator, f.initErr
}
