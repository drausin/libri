package pagination

import (
	"bytes"
	"github.com/drausin/libri/libri/author/io/compression"
	"github.com/drausin/libri/libri/author/io/encryption"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"github.com/drausin/libri/libri/common/ecid"
	"fmt"
)

func TestPaginateUnpaginate(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, authorID := encryption.NewPseudoRandomKeys(rng), ecid.NewPseudoRandom(rng)

	encrypter, err := encryption.NewEncrypter(keys)
	assert.Nil(t, err)

	decrypter, err := encryption.NewDecrypter(keys)
	assert.Nil(t, err)

	//uncompressedSizes := []int{128, 192, 256, 384, 512, 768, 1024}
	uncompressedSizes := []int{256}
	//pageSizes := []uint32{128, 256, 512, 1024}
	pageSizes := []uint32{128}
	//mediaTypes := []string{"application/x-pdf", "application/x-gzip"}
	mediaTypes := []string{"application/x-pdf"}

	for _, c := range caseCrossProduct(pageSizes, uncompressedSizes, mediaTypes) {
		pages := make(chan *api.Page, 3)
		paginator, err := NewPaginator(pages, encrypter, authorID, keys.PageHMACKey,
			c.pageSize)
		assert.Nil(t, err)

		uncompressed1 := newTestBytes(rng, c.uncompressedSize)
		uncompressed1Bytes := uncompressed1.Bytes()

		uncompressedBufferSize := int(c.pageSize)
		compressor, err := compression.NewCompressor(uncompressed1, c.mediaType,
			uncompressedBufferSize)
		assert.Nil(t, err)

		uncompressed2 := new(bytes.Buffer)
		decompressor, err := compression.NewDecompressor(uncompressed2, c.mediaType,
			uncompressedBufferSize)
		assert.Nil(t, err)

		unpaginator, err := NewUnpaginator(pages, decrypter, keys.PageHMACKey)
		assert.Nil(t, err)

		// test writing and reading in parallel
		go func() {
			_, err = paginator.ReadFrom(compressor)
			assert.Nil(t, err, c.String())
			close(pages)
		}()

		_, err = unpaginator.WriteTo(decompressor)
		assert.Nil(t, err)
		assert.Equal(t, c.uncompressedSize, uncompressed2.Len())
		assert.Equal(t, uncompressed1Bytes, uncompressed2.Bytes(), c.String())
	}
}


type pageTestCase struct {
	pageSize uint32
	uncompressedSize int
	mediaType string
}

func (p pageTestCase) String() string {
	return fmt.Sprintf("pageSize: %d, uncompressedSize: %d, mediaType: %s", p.pageSize,
		p.uncompressedSize, p.mediaType)
}

func caseCrossProduct(
	pageSizes []uint32, uncompressedSizes []int, mediaTypes []string,
) ([]*pageTestCase) {
	cases := make([]*pageTestCase, 0)
	for _, pageSize := range pageSizes {
		for _, uncompressedSize := range uncompressedSizes {
			for _, mediaType := range mediaTypes {
				cases = append(cases, &pageTestCase{
					pageSize: pageSize,
					uncompressedSize: uncompressedSize,
					mediaType: mediaType,
				})
			}
		}
	}
	return cases
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
