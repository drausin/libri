package page

import (
	"math/rand"
	"testing"

	"errors"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/common/storage"
)

func TestStorerLoader_Store_ok(t *testing.T) {
	sl := NewStorerLoader(
		&storage.TestDocSLD{
			Stored: make(map[string]*api.Document),
		},
	)
	rng := rand.New(rand.NewSource(0))
	nPages := 4
	pages := make(chan *api.Page, nPages)
	for c := 0; c < nPages; c++ {
		pages <- api.NewTestPage(rng)
	}
	close(pages)

	// check we have expected number of page IDs
	pageIDs, err := sl.Store(pages)
	assert.Nil(t, err)
	assert.Equal(t, nPages, len(pageIDs))
}

func TestStorerLoader_Store_err(t *testing.T) {
	sl := NewStorerLoader(&storage.TestDocSLD{StoreErr: errors.New("some Store error")})
	rng := rand.New(rand.NewSource(0))
	pages := make(chan *api.Page, 1)
	pages <- api.NewTestPage(rng)
	close(pages)

	// check inner store error bubbles up
	pageIDs, err := sl.Store(pages)
	assert.NotNil(t, err)
	assert.Nil(t, pageIDs)
}

func TestStorerLoader_Load_ok(t *testing.T) {
	stored := make(map[string]*api.Document)
	rng := rand.New(rand.NewSource(0))
	nPages := 4
	pageIDs := make([]id.ID, nPages)
	for c := 0; c < nPages; c++ {
		page := api.NewTestPage(rng)
		pageID, err := api.GetKey(page)
		assert.Nil(t, err)
		pageIDs[c] = pageID
		stored[pageID.String()] = &api.Document{
			Contents: &api.Document_Page{Page: page},
		}
	}

	sl := NewStorerLoader(
		&storage.TestDocSLD{
			Stored: stored,
		},
	)

	pages, abort := make(chan *api.Page, nPages), make(chan struct{})
	err := sl.Load(pageIDs, pages, abort)
	assert.Nil(t, err)
	close(pages)

	// check we get the right number of pages and that their order is same as key order
	i := 0
	for page := range pages {
		pageID, err := api.GetKey(page)
		assert.Nil(t, err)
		assert.Equal(t, pageIDs[i], pageID)
		i++
	}
	assert.Equal(t, nPages, i)

	// check abort channel terminates load
	pages, abort = make(chan *api.Page, nPages), make(chan struct{})
	close(abort)
	close(pages)
	err = sl.Load(pageIDs, pages, abort)
	assert.Nil(t, err)
}

func TestStorerLoader_Load_err(t *testing.T) {
	sl1 := NewStorerLoader(&storage.TestDocSLD{LoadErr: errors.New("some Load error")})
	rng := rand.New(rand.NewSource(0))
	pageIDs1 := []id.ID{id.NewPseudoRandom(rng)}

	// check inner load error bubbles up
	err := sl1.Load(pageIDs1, nil, make(chan struct{}))
	assert.NotNil(t, err)

	sl2 := NewStorerLoader(
		&storage.TestDocSLD{
			Stored: make(map[string]*api.Document),
		},
	)
	pageIDs2 := []id.ID{id.NewPseudoRandom(rng)}

	// check error on missing doc
	err = sl2.Load(pageIDs2, nil, make(chan struct{}))
	assert.NotNil(t, err)

	stored := make(map[string]*api.Document)
	notPage, pageID := api.NewTestDocument(rng)
	stored[pageID.String()] = notPage
	pageIDs3 := []id.ID{pageID}

	sl3 := NewStorerLoader(
		&storage.TestDocSLD{
			Stored: stored,
		},
	)

	// check error returned from non-Page document
	err = sl3.Load(pageIDs3, nil, make(chan struct{}))
	assert.NotNil(t, err)
}

func TestStorerLoader_StoreLoad(t *testing.T) {
	sl := NewStorerLoader(
		&storage.TestDocSLD{
			Stored: make(map[string]*api.Document),
		},
	)
	rng := rand.New(rand.NewSource(0))
	nPages := 4
	originalPages := make([]*api.Page, nPages)
	pagesToStore := make(chan *api.Page, nPages)
	for i := 0; i < nPages; i++ {
		originalPages[i] = api.NewTestPage(rng)
		pagesToStore <- originalPages[i]
	}
	close(pagesToStore)

	pageIDs, err := sl.Store(pagesToStore)
	assert.Nil(t, err)
	assert.Equal(t, nPages, len(pageIDs))

	pagesToLoad := make(chan *api.Page, nPages)
	err = sl.Load(pageIDs, pagesToLoad, make(chan struct{}))
	assert.Nil(t, err)
	close(pagesToLoad)

	loadedPages := make([]*api.Page, nPages)
	for i := 0; i < nPages; i++ {
		loadedPages[i] = <-pagesToLoad
	}

	// check original and loaded pags are in the same order and are equal
	assert.Equal(t, originalPages, loadedPages)
}
