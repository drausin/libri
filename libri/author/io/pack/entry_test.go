package pack

import (
	"testing"
	"github.com/drausin/libri/libri/author/io/print"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/id"
	"bytes"
	"math/rand"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/pkg/errors"
)

func TestEntryPacker_Pack_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := print.NewDefaultParameters()
	page.MinSize = 64  // just for testing
	params.PageSize = 128
	docSL := &fixedDocStorerLoader{
		stored: make(map[string]*api.Document),
	}
	p := NewEntryPacker(params, enc.NewMetadataEncrypterDecrypter(), docSL)
	keys, authorPub, _ := enc.NewPseudoRandomKeys(rng)

	// test works with single-page content
	content, mediaType := newTestBytes(rng, int(params.PageSize  / 2)), "application/x-pdf"
	doc, pageKeys, err := p.Pack(content, mediaType, keys, authorPub)
	assert.Nil(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, 1, len(pageKeys))

	// test works with multi-page content
	content, mediaType = newTestBytes(rng, int(params.PageSize  * 5)), "application/x-pdf"
	doc, pageKeys, err = p.Pack(content, mediaType, keys, authorPub)
	assert.Nil(t, err)
	assert.NotNil(t, doc)
	assert.True(t, len(pageKeys) > 1)

	// check doc page keys line up with those returned by Pack
	docPageKeys := doc.Contents.(*api.Document_Entry).Entry.Contents.
		(*api.Entry_PageKeys).PageKeys.Keys
	assert.Equal(t, len(docPageKeys), len(pageKeys))
	for i, pageKey := range pageKeys {
		assert.Equal(t, pageKey.Bytes(), docPageKeys[i])
	}
}

func TestEntryPacker_Pack_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := print.NewDefaultParameters()
	docSL := &fixedDocStorerLoader{
		stored: make(map[string]*api.Document),
	}
	p := NewEntryPacker(params, enc.NewMetadataEncrypterDecrypter(), docSL)
	content, mediaType := newTestBytes(rng, int(params.PageSize  / 2)), "application/x-pdf"
	keys, authorPub, _ := enc.NewPseudoRandomKeys(rng)

	// check error from bad mediaType bubbles up
	doc, pageKeys, err := p.Pack(content, "application x-pdf", keys, authorPub)
	assert.NotNil(t, err)
	assert.Nil(t, doc)
	assert.Nil(t, pageKeys)

	// check Encrypt error from bad author key bubbles up
	doc, pageKeys, err = p.Pack(content, mediaType, keys, []byte{})
	assert.NotNil(t, err)
	assert.Nil(t, doc)
	assert.Nil(t, pageKeys)

	errDocSL := &fixedDocStorerLoader{
		stored: make(map[string]*api.Document),
		loadErr: errors.New("some Load error"),
	}
	p2 := NewEntryPacker(params, enc.NewMetadataEncrypterDecrypter(), errDocSL)

	// check error from missing page bubbles up
	doc, pageKeys, err = p2.Pack(content, mediaType, keys, []byte{})
	assert.NotNil(t, err)
	assert.Nil(t, doc)
	assert.Nil(t, pageKeys)

}

type fixedDocStorerLoader struct {
	storeErr error
	stored map[string]*api.Document
	loadErr error
}

func (f *fixedDocStorerLoader) Store(key id.ID, value *api.Document) error {
	f.stored[key.String()] = value
	return f.storeErr
}

func (f *fixedDocStorerLoader) Load(key id.ID) (*api.Document, error) {
	value, _ := f.stored[key.String()]
	return value, f.loadErr
}

func newTestBytes(rng *rand.Rand, size int) *bytes.Buffer {
	dict := []string{
		"these", "are", "some", "test", "words", "that", "will", "be", "compressed",
	}
	words := new(bytes.Buffer)
	for {
		word := dict[int(rng.Int31n(int32(len(dict))))] + " "
		if words.Len()+len(word) > size {
			// pad words to exact length
			words.Write(make([]byte, size-words.Len()))
			break
		}
		words.WriteString(word)
	}

	return words
}