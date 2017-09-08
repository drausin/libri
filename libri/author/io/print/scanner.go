package print

import (
	"io"
	"sync"

	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// Scanner writes locally-stored pages to a unified content stream.
type Scanner interface {
	// Scan loads pages with the given keys and metadata from an internal page.Loader and
	// writes their concatenated output to the content io.Writer.
	Scan(content io.Writer, pageKeys []id.ID, keys *enc.EEK, metatdata *api.EntryMetadata) error
}

type scanner struct {
	params *Parameters
	pageL  page.Loader
	init   scanInitializer
}

// NewScanner creates a new Scanner object with the given parameters, encryption keys, and page
// loader.
func NewScanner(params *Parameters, pageL page.Loader) Scanner {
	return &scanner{
		params: params,
		pageL:  pageL,
		init: &scanInitializerImpl{
			params: params,
		},
	}
}

func (s *scanner) Scan(
	content io.Writer, pageKeys []id.ID, keys *enc.EEK, md *api.EntryMetadata,
) error {

	pages := make(chan *api.Page, int(s.params.Parallelism))
	if err := api.ValidateEntryMetadata(md); err != nil {
		return err
	}
	decompressor, unpaginator, err := s.init.Initialize(content, md.CompressionCodec, keys, pages)
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

	return enc.CheckMACs(unpaginator.CiphertextMAC(), decompressor.UncompressedMAC(), md)
}

type scanInitializer interface {
	Initialize(content io.Writer, codec api.CompressionCodec, keys *enc.EEK, pages chan *api.Page) (
		comp.Decompressor, page.Unpaginator, error)
}

type scanInitializerImpl struct {
	params *Parameters
}

func (si *scanInitializerImpl) Initialize(
	content io.Writer, codec api.CompressionCodec, keys *enc.EEK, pages chan *api.Page,
) (comp.Decompressor, page.Unpaginator, error) {

	decompressor, err := comp.NewDecompressor(content, codec, keys,
		si.params.CompressionBufferSize)
	if err != nil {
		return nil, nil, err
	}
	decrypter, err := enc.NewDecrypter(keys)
	if err != nil {
		return nil, nil, err
	}
	unpaginator, err := page.NewUnpaginator(pages, decrypter, keys)
	if err != nil {
		return nil, nil, err
	}
	return decompressor, unpaginator, nil
}
