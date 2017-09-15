package print

import (
	"errors"
	"io"

	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

const (
	// DefaultParallelism is the default Print and Scan parallelism.
	DefaultParallelism = uint32(3)
)

// ErrZeroParallelism indicates when Print and Scan parallelism is improperly set to zero.
var ErrZeroParallelism = errors.New("zero value parallelism")

// Parameters define various parameters used by Printers and Scanners.
type Parameters struct {
	// CompressionBufferSize is the size of the internal buffer used by comp.Compressors and
	// comp.Decompressors.
	CompressionBufferSize uint32

	// PageSize is the maximum size (in bytes) of an api.Page ciphertext.
	PageSize uint32

	// Parallelism is the parallelism used by Printers and Scanners when storing and loading
	// pages.
	Parallelism uint32
}

// NewParameters creates a new *Parameters instance.
func NewParameters(
	compressionBufferSize uint32, pageSize uint32, parallelism uint32,
) (*Parameters, error) {
	if parallelism == 0 {
		return nil, ErrZeroParallelism
	}
	return &Parameters{
		CompressionBufferSize: compressionBufferSize,
		PageSize:              pageSize,
		Parallelism:           parallelism,
	}, nil
}

// NewDefaultParameters creates a default *Parameters instance.
func NewDefaultParameters() *Parameters {
	params, err := NewParameters(comp.DefaultBufferSize, page.DefaultSize, DefaultParallelism)
	cerrors.MaybePanic(err) // should never happen; if does, it's programmer error
	return params
}

// Printer stores pages created from (uncompressed) content.
type Printer interface {
	// Print creates pages from the given content and stores them via an internal page.Storer.
	Print(content io.Reader, mediaType string, keys *enc.EEK, authorPub []byte) (
		[]id.ID, *api.EntryMetadata, error)
}

type printer struct {
	params *Parameters
	pageS  page.Storer
	init   printInitializer
}

// NewPrinter returns a new Printer instance.
func NewPrinter(
	params *Parameters,
	pageS page.Storer,
) Printer {
	return &printer{
		params: params,
		pageS:  pageS,
		init: &printInitializerImpl{
			params: params,
		},
	}
}

func (p *printer) Print(content io.Reader, mediaType string, keys *enc.EEK, authorPub []byte) (
	[]id.ID, *api.EntryMetadata, error) {

	pages := make(chan *api.Page, int(p.params.Parallelism))
	codec, err := comp.GetCompressionCodec(mediaType)
	if err != nil {
		return nil, nil, err
	}
	compressor, paginator, err := p.init.Initialize(content, codec, keys, authorPub, pages)
	if err != nil {
		return nil, nil, err
	}
	errs := make(chan error, 1)
	go func() {
		_, rfErr := paginator.ReadFrom(compressor)
		if rfErr != nil {
			errs <- rfErr
		}
		close(pages)
	}()

	pageKeys, err := p.pageS.Store(pages)
	if err != nil {
		return nil, nil, err
	}

	select {
	case err = <-errs:
		return nil, nil, err
	default:
	}

	metadata := &api.EntryMetadata{
		MediaType:        mediaType,
		CompressionCodec: codec,
		CiphertextSize:   paginator.CiphertextMAC().MessageSize(),
		CiphertextMac:    paginator.CiphertextMAC().Sum(nil),
		UncompressedSize: compressor.UncompressedMAC().MessageSize(),
		UncompressedMac:  compressor.UncompressedMAC().Sum(nil),
	}
	if err := api.ValidateEntryMetadata(metadata); err != nil {
		return nil, nil, err
	}

	return pageKeys, metadata, nil
}

type printInitializer interface {
	Initialize(content io.Reader, codec api.CompressionCodec, keys *enc.EEK, authorPub []byte,
		pages chan *api.Page) (comp.Compressor, page.Paginator, error)
}

type printInitializerImpl struct {
	params *Parameters
}

func (pi *printInitializerImpl) Initialize(
	content io.Reader,
	codec api.CompressionCodec,
	keys *enc.EEK,
	authorPub []byte,
	pages chan *api.Page,
) (comp.Compressor, page.Paginator, error) {

	compressor, err := comp.NewCompressor(content, codec, keys,
		pi.params.CompressionBufferSize)
	if err != nil {
		return nil, nil, err
	}
	encrypter, err := enc.NewEncrypter(keys)
	if err != nil {
		return nil, nil, err
	}
	paginator, err := page.NewPaginator(pages, encrypter, keys, authorPub,
		pi.params.PageSize)
	if err != nil {
		return nil, nil, err
	}
	return compressor, paginator, nil
}
