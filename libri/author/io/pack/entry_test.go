package pack

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/io/page"
	"github.com/drausin/libri/libri/author/io/print"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestEntryPacker_Pack_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := print.NewDefaultParameters()
	page.MinSize = 64 // just for testing
	params.PageSize = 128
	docSL := storage.NewTestDocSLD()
	p := NewEntryPacker(params, enc.NewMetadataEncrypterDecrypter(), docSL)
	authorPub := api.RandBytes(rng, 65)
	keys := enc.NewPseudoRandomEEK(rng)
	mediaType := "application/x-pdf"

	// test works with single-page content
	uncompressedSize1 := int(params.PageSize / 2)
	content1 := common.NewCompressableBytes(rng, uncompressedSize1)
	doc, metadata, err := p.Pack(content1, mediaType, keys, authorPub)
	assert.Nil(t, err)
	assert.NotNil(t, doc)
	assert.NotNil(t, metadata)
	assert.Equal(t, uint64(uncompressedSize1), metadata.UncompressedSize)

	// test works with multi-page content
	uncompressedSize2 := int(params.PageSize * 5)
	content2 := common.NewCompressableBytes(rng, uncompressedSize2)
	doc, metadata, err = p.Pack(content2, mediaType, keys, authorPub)
	assert.Nil(t, err)
	assert.NotNil(t, doc)
	assert.NotNil(t, metadata)
	assert.Equal(t, uint64(uncompressedSize2), metadata.UncompressedSize)
	pageKeys, err := api.GetEntryPageKeys(doc)
	assert.Nil(t, err)
	assert.True(t, len(pageKeys) > 1)
}

func TestEntryPacker_Pack_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := print.NewDefaultParameters()
	docSL := &storage.TestDocSLD{
		Stored: make(map[string]*api.Document),
	}
	p := NewEntryPacker(params, enc.NewMetadataEncrypterDecrypter(), docSL)
	mediaType := "application/x-pdf"
	content := common.NewCompressableBytes(rng, int(params.PageSize/2))
	authorPub := api.RandBytes(rng, 65)
	keys := enc.NewPseudoRandomEEK(rng)

	// check error from bad mediaType bubbles up
	doc, metadata, err := p.Pack(content, "application x-pdf", keys, authorPub)
	assert.NotNil(t, err)
	assert.Nil(t, doc)
	assert.Nil(t, metadata)

	// check Encrypt error from bad author key bubbles up
	doc, metadata, err = p.Pack(content, mediaType, keys, []byte{})
	assert.NotNil(t, err)
	assert.Nil(t, doc)
	assert.Nil(t, metadata)

	errDocSL := &storage.TestDocSLD{
		Stored:  make(map[string]*api.Document),
		LoadErr: errors.New("some Load error"),
	}
	p2 := NewEntryPacker(params, enc.NewMetadataEncrypterDecrypter(), errDocSL)

	// check error from missing page bubbles up
	doc, metadata, err = p2.Pack(content, mediaType, keys, []byte{})
	assert.NotNil(t, err)
	assert.Nil(t, doc)
	assert.Nil(t, metadata)

}

func TestEntryUnpacker_Unpack_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := print.NewDefaultParameters()
	docSL := &storage.TestDocSLD{
		Stored: make(map[string]*api.Document),
	}
	keys := enc.NewPseudoRandomEEK(rng)
	content := new(bytes.Buffer)
	doc, _ := api.NewTestDocument(rng)
	metadata1 := &api.EntryMetadata{
		MediaType:        "application/x-pdf",
		CiphertextSize:   1,
		CiphertextMac:    api.RandBytes(rng, 32),
		UncompressedSize: 2,
		UncompressedMac:  api.RandBytes(rng, 32),
	}

	metadataDec := &fixedMetadataDecrypter{
		metadata: metadata1,
	}
	u := NewEntryUnpacker(params, metadataDec, docSL)
	u.(*entryUnpacker).scanner = &fixedScanner{}
	metadata, err := u.Unpack(content, doc, keys)
	assert.Nil(t, err)
	assert.NotNil(t, metadata)
}

func TestEntryUnpacker_Unpack_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	params := print.NewDefaultParameters()
	docSL := &storage.TestDocSLD{
		Stored: make(map[string]*api.Document),
	}
	content := new(bytes.Buffer)
	doc, _ := api.NewTestDocument(rng)
	keys := enc.NewPseudoRandomEEK(rng)

	// check bad ciphertext/ciphertext MAC trigger error
	u1 := NewEntryUnpacker(params, &fixedMetadataDecrypter{}, docSL)
	doc1, _ := api.NewTestDocument(rng)
	doc1.Contents.(*api.Document_Entry).Entry.MetadataCiphertextMac = nil
	metadata, err := u1.Unpack(content, doc1, keys)
	assert.NotNil(t, err)
	assert.Nil(t, metadata)

	// check decryption error bubbles up
	u2 := NewEntryUnpacker(
		params,
		&fixedMetadataDecrypter{err: errors.New("some Decrypt error")},
		docSL,
	)
	metadata, err = u2.Unpack(content, doc, keys)
	assert.NotNil(t, err)
	assert.Nil(t, metadata)

	// check scanner error bubbles up
	u3 := NewEntryUnpacker(params, &fixedMetadataDecrypter{}, docSL)
	u3.(*entryUnpacker).scanner = &fixedScanner{
		err: errors.New("some Scan error"),
	}
	metadata, err = u3.Unpack(content, doc, keys)
	assert.NotNil(t, err)
	assert.Nil(t, metadata)
}

func TestEntryPackUnpack(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	page.MinSize = 64 // just for testing
	authorPub := api.RandBytes(rng, 65)
	keys := enc.NewPseudoRandomEEK(rng)
	metadataEncDec := enc.NewMetadataEncrypterDecrypter()

	pageSizes := []uint32{128, 256, 512, 1024}
	uncompressedSizes := []int{128, 192, 256, 384, 512, 768, 1024, 2048, 4096, 8192}
	mediaTypes := []string{"application/x-pdf", "application/x-gzip"}
	packParallelisms := []uint32{1, 2, 3}
	unpackParallelisms := []uint32{1, 2, 3}
	cases := caseCrossProduct(
		pageSizes,
		uncompressedSizes,
		mediaTypes,
		packParallelisms,
		unpackParallelisms,
	)

	for _, c := range cases {
		content1 := common.NewCompressableBytes(rng, c.uncompressedSize)
		content1Bytes := content1.Bytes()
		docSL := storage.NewTestDocSLD()
		packParams, err := print.NewParameters(comp.MinBufferSize, c.pageSize,
			c.packParallelism)
		assert.Nil(t, err)
		p := NewEntryPacker(packParams, metadataEncDec, docSL)
		unpackParams, err := print.NewParameters(comp.MinBufferSize, c.pageSize,
			c.unpackParallelism)
		assert.Nil(t, err)
		u := NewEntryUnpacker(unpackParams, metadataEncDec, docSL)

		doc, metadata1, err := p.Pack(content1, c.mediaType, keys, authorPub)
		assert.Nil(t, err)
		assert.NotNil(t, doc)
		assert.Equal(t, c.uncompressedSize, int(metadata1.UncompressedSize))

		content2 := new(bytes.Buffer)
		metadata2, err := u.Unpack(content2, doc, keys)
		assert.Nil(t, err)
		assert.Equal(t, content1Bytes, content2.Bytes())
		assert.Equal(t, c.uncompressedSize, int(metadata2.UncompressedSize))
	}
}

type fixedMetadataDecrypter struct {
	metadata *api.EntryMetadata
	err      error
}

func (f *fixedMetadataDecrypter) Decrypt(em *enc.EncryptedMetadata, keys *enc.EEK) (
	*api.EntryMetadata, error) {
	return f.metadata, f.err
}

type fixedScanner struct {
	err error
}

func (f *fixedScanner) Scan(
	content io.Writer, pageKeys []id.ID, keys *enc.EEK, metatdata *api.EntryMetadata,
) error {
	return f.err
}

type packTestCase struct {
	pageSize          uint32
	uncompressedSize  int
	mediaType         string
	packParallelism   uint32
	unpackParallelism uint32
}

func caseCrossProduct(
	pageSizes []uint32,
	uncompressedSizes []int,
	mediaTypes []string,
	packParallelisms []uint32,
	unpackParallelisms []uint32,
) []*packTestCase {
	cases := make([]*packTestCase, 0)
	for _, pageSize := range pageSizes {
		for _, uncompressedSize := range uncompressedSizes {
			for _, mediaType := range mediaTypes {
				for _, packParallelism := range packParallelisms {
					for _, unpackParallelism := range unpackParallelisms {
						cases = append(cases, &packTestCase{
							pageSize:          pageSize,
							uncompressedSize:  uncompressedSize,
							mediaType:         mediaType,
							packParallelism:   packParallelism,
							unpackParallelism: unpackParallelism,
						})
					}
				}
			}
		}
	}
	return cases
}
