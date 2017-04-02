package pagination

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
)

type StorerLoader interface {
	// Store writes pages to local storage and returns a slice of their keys.
	Store(pages chan *api.Page) ([]id.ID, error)
}

type storerLoader struct {
	inner storage.DocumentStorerLoader
}

func NewStorerLoader(inner storage.DocumentStorerLoader) StorerLoader {
	return &storerLoader{
		inner: inner,
	}
}

func (s *storerLoader) Store(pages chan *api.Page) ([]id.ID, error) {
	keys := make([]id.ID, 0)
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

func (s *storerLoader) Load(keys []id.ID, pages chan *api.Page) error {
	// TODO (drausin)
	return nil
}
