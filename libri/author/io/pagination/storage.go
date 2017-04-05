package pagination

import (
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
)

// ErrUnexpectedDocContent indicates that a document is not of the expected content type (e.g.,
// an Entry when expecing a Page).
var ErrUnexpectedDocContent = errors.New("unexpected document content")

// ErrMissingPage indicates when a page was expected to be stored but was not found.
var ErrMissingPage = errors.New("missing page")

// StorerLoader stores and loads pages from an inner storage.DocumentStorerLoader.
type StorerLoader interface {
	// Store writes pages to inner storage and returns a slice of their keys.
	Store(pages chan *api.Page) ([]cid.ID, error)

	// Load reads pages from inner storage and sends them on the supplied pages channel.
	Load(keys []cid.ID, pages chan *api.Page) error
}

type storerLoader struct {
	inner storage.DocumentStorerLoader
}

// NewStorerLoader creates a new StorerLoader instance from an inner storage.DocumentStorerLoader
// instance.
func NewStorerLoader(inner storage.DocumentStorerLoader) StorerLoader {
	return &storerLoader{
		inner: inner,
	}
}

func (s *storerLoader) Store(pages chan *api.Page) ([]cid.ID, error) {
	keys := make([]cid.ID, 0)
	for page := range pages {
		doc := &api.Document{
			Contents: &api.Document_Page{Page: page},
		}
		key, err := api.GetKey(doc)
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

func (s *storerLoader) Load(keys []cid.ID, pages chan *api.Page) error {
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
		pages <- docPage.Page
	}
	return nil
}
