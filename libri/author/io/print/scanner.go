package print

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"io"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/io/comp"
	"sync"
)

// Scanner writes locally-stored pages to a unified content stream.
type Scanner interface {
	// Scan loads pages with the given keys and metadata from an internal page.Loader and
	// writes their concatenated output to the content io.Writer.
	Scan(content io.Writer, pageKeys []id.ID, metatdata *api.Metadata) error
}

type scanner struct {
	params *Parameters
	keys *enc.Keys
	pageL page.Loader
	init scanInitializer
}

// NewScanner creates a new Scanner object with the given parameters, encryption keys, and page
// loader.
func NewScanner (
	params *Parameters,
	keys *enc.Keys,
	pageL page.Loader,
) Scanner {
	return &scanner {
		params:    params,
		keys:      keys,
		pageL:     pageL,
		init: &scanInitializerImpl{
			keys:      keys,
			params:    params,
		},
	}
}

func (s *scanner) Scan(content io.Writer, pageKeys []id.ID, md *api.Metadata) error {
	pages := make(chan *api.Page, int(s.params.Parallelism))
	if err := api.ValidateMetadata(md); err != nil {
		return err
	}
	mediaType, _ := md.GetMediaType()
	decompressor, unpaginator, err := s.init.Initialize(content, mediaType, pages)
	if err != nil {
		return err
	}
	errs := make(chan error, 1)
	abortLoad := make(chan struct{})
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		_, wtErr := unpaginator.WriteTo(decompressor)
		if wtErr != nil {
			errs <- wtErr
			close(abortLoad)
		}
		wg.Done()
	}()

	err = s.pageL.Load(pageKeys, pages, abortLoad)
	close(pages)
	if err != nil {
		return err
	}
	wg.Wait()

	select {
	case err = <-errs:
		return err
	default:
	}

	if err := enc.CheckMACs(unpaginator.CiphertextMAC(), decompressor.UncompressedMAC(),
		md); err != nil {
		return err
	}
	return nil
}

type scanInitializer interface {
	Initialize(content io.Writer, mediaType string, pages chan *api.Page) (
		comp.Decompressor, page.Unpaginator, error)
}

type scanInitializerImpl struct {
	keys *enc.Keys
	params *Parameters
}

func (si *scanInitializerImpl) Initialize(
	content io.Writer, mediaType string, pages chan *api.Page,
) (comp.Decompressor, page.Unpaginator, error) {

	codec, err := comp.GetCompressionCodec(mediaType)
	if err != nil {
		return nil, nil, err
	}
	decompressor, err := comp.NewDecompressor(content, codec, si.keys,
		si.params.CompressionBufferSize)
	if err != nil {
		return nil, nil, err
	}
	decrypter, err := enc.NewDecrypter(si.keys)
	if err != nil {
		return nil, nil, err
	}
	unpaginator, err := page.NewUnpaginator(pages, decrypter, si.keys)
	if err != nil {
		return nil, nil, err
	}
	return decompressor, unpaginator, nil
}
