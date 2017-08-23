package page

import (
	"errors"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
)

var (

	// ErrUnexpectedDocContent indicates that a document is not of the expected content type
	// (e.g., an Entry when expecing a Page).
	ErrUnexpectedDocContent = errors.New("unexpected document content")

	// ErrMissingPage indicates when a page was expected to be stored but was not found.
	ErrMissingPage = errors.New("missing page")
)

// Storer stores pages to an inner storage.DocumentStorer.
type Storer interface {
	// Store writes pages to inner storage and returns a slice of their keys.
	Store(pages chan *api.Page) ([]id.ID, error)
}

// Loader loads pages from an inner storage.DocmentLoader.
type Loader interface {
	// Load reads pages from inner storage and sends them on the supplied pages channel. A
	// signal (or closing) on the abort channel interrupts the loading.
	Load(keys []id.ID, pages chan *api.Page, abort chan struct{}) error
}

// StorerLoader stores and loads pages from an inner storage.DocumentStorerLoader.
type StorerLoader interface {
	Storer
	Loader
}

type storerLoader struct {
	inner storage.DocumentSLD
}

// NewStorerLoader creates a new StorerLoader instance from an inner storage.DocumentSLD
// instance.
func NewStorerLoader(inner storage.DocumentSLD) StorerLoader {
	return &storerLoader{
		inner: inner,
	}
}

func (s *storerLoader) Store(pages chan *api.Page) ([]id.ID, error) {
	keys := make([]id.ID, 0)
	for page := range pages {
		doc, key, err := api.GetPageDocument(page)
		if err != nil {
			return nil, err
		}
		if err := s.inner.Store(key, doc); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *storerLoader) Load(keys []id.ID, pages chan *api.Page, abort chan struct{}) error {
	for _, key := range keys {
		doc, err := s.inner.Load(key)
		if err != nil {
			return err
		}
		if doc == nil {
			return ErrMissingPage
		}
		docPage, ok := doc.Contents.(*api.Document_Page)
		if !ok {
			return ErrUnexpectedDocContent
		}
		select {
		case <-abort:
			return nil
		default:
			pages <- docPage.Page
		}
	}
	return nil
}
