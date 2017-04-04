package pagination

import (
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
)

var UnexpectedDocContentErr = errors.New("unexpected document content")
var MissingPageErr = errors.New("missing page")

type StorerLoader interface {
	// Store writes pages to inner storage and returns a slice of their keys.
	Store(pages chan *api.Page) ([]cid.ID, error)

	// Load reads pages from inner storage and sends them on the supplied pages channel.
	Load(keys []cid.ID, pages chan *api.Page) error
}

type storerLoader struct {
	inner storage.DocumentStorerLoader
}

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
			return MissingPageErr
		}
		docPage, ok := doc.Contents.(*api.Document_Page)
		if !ok {
			return UnexpectedDocContentErr
		}
		pages <- docPage.Page
	}
	return nil
}
