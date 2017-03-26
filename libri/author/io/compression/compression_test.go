package compression

import (
	"testing"
	"math/rand"
	"github.com/stretchr/testify/assert"
	"bytes"
	"fmt"
	"io"
	"compress/gzip"
)


func TestCompressDecompress(t *testing.T) {
	mediaCases := []mediaTestCase{
		{"application/pdf", false},
		{"application/x-gzip", true},  // equalSize since we're not compressing twice
	}
	uncompressedSizes := []int{128, 192, 256, 384, 512, 768, 1024}
	uncompressedBufferSizes := []int{128, 192, 256, 384, 512, 768, 1024}
	compressedBufferSizes := []int{128, 192, 256, 384, 512, 768, 1024}
	cases := caseCrossProduct(
		uncompressedSizes,
		uncompressedBufferSizes,
		compressedBufferSizes,
		mediaCases,
	)

	rng := rand.New(rand.NewSource(0))
	for _, c := range cases {
		uncompressed1 := newTestBytes(rng, c.uncompressedSize)
		uncompressed1Bytes := uncompressed1.Bytes()
		assert.Equal(t, c.uncompressedSize, uncompressed1.Len())

		compressor, err := NewCompressor(
			uncompressed1,
			c.media.mediaType,
			c.uncompressedBufferSize,
		)
		assert.Nil(t, err, c.String())

		// get the compressed bytes
		compressed := new(bytes.Buffer)
		n1 := c.compressedBufferSize
		for n1 == c.compressedBufferSize {
			buf := make([]byte, c.compressedBufferSize)
			n1, err = compressor.Read(buf)
			assert.True(t, err == nil || err == io.EOF, c.String())
			compressed.Write(buf[:n1])
		}

		if !c.media.equalSize {
			// make sure compression has actually happened
			assert.True(t, compressed.Len() < c.uncompressedSize, c.String())
		}

		uncompressed2 := new(bytes.Buffer)
		decompressorImpl, err := NewDecompressor(
			uncompressed2,
			c.media.mediaType,
			c.uncompressedBufferSize,
		)
		assert.Nil(t, err, c.String())

		compressedLen := compressed.Len()
		n2, err := decompressorImpl.Write(compressed.Bytes())
		assert.Equal(t, compressedLen, n2)
		assert.Nil(t, err)
		assert.Equal(t, uncompressed1Bytes, uncompressed2.Bytes(), c.String())
	}
}

func TestGetCompressionCodec(t *testing.T) {
	// check handles empty string media type ok
	c1, err := GetCompressionCodec("")
	assert.Equal(t, DefaultCodec, c1)
	assert.Nil(t, err)

	// check not-nil err on bad media type
	c2, err := GetCompressionCodec("/blah")
	assert.Equal(t, DefaultCodec, c2)
	assert.NotNil(t, err)

	// check don't compress something already compressed
	c3, err := GetCompressionCodec("application/x-gzip")
	assert.Equal(t, NoneCodec, c3)
	assert.Nil(t, err)

	// check default codec
	c4, err := GetCompressionCodec("application/pdf")
	assert.Equal(t, GZIPCodec, c4)
	assert.Nil(t, err)

}

func TestWriteNBytes(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	for writeSize := 16; writeSize < 32 ; writeSize *= 2 {
		for tol := float32(0.8); tol < 0.9; tol += (1.0 - tol) * 0.5 {
			info := fmt.Sprintf("compressedSize: %d, tolBelow: %d", writeSize,
				tol)
			from := newTestBytes(rng, writeSize * 10)  // assumed enough
			wBuf := new(bytes.Buffer)
			w, err := gzip.NewWriterLevel(wBuf, gzip.BestCompression)
			assert.Nil(t, err)

			n, err := writeNBytes(from, w, wBuf, writeSize, tol)
			assert.Nil(t, err, info)
			assert.True(t, int(float32(writeSize) * tol) <= n, info)
			assert.True(t, n <= writeSize, info)
		}
	}
}


type mediaTestCase struct {
	mediaType string
	equalSize bool
}

type compressionTestCase struct {
	uncompressedSize int
	uncompressedBufferSize int
	compressedBufferSize int
	media mediaTestCase
}

func (c compressionTestCase) String() string {
	return fmt.Sprintf(
		"uncompressedSize: %d, uncompressedBufferSize: %d, " +
		"compressedBufferSize: %d, mediaType: %s, equalSize: %v",
		c.uncompressedSize,
		c.uncompressedBufferSize,
		c.compressedBufferSize,
		c.media.mediaType,
		c.media.equalSize,
	)
}

func caseCrossProduct(
	uncompressedSizes []int,
	uncompressedBufferSizes []int,
	compressedBufferSizes []int,
	mediaCases []mediaTestCase,
) []compressionTestCase {
	cases := make([]compressionTestCase, 0)
	for _, uncompressedSize := range uncompressedSizes {
		for _, uncompressedBufferSize := range uncompressedBufferSizes {
			for _, compressedBufferSize := range compressedBufferSizes {
				for _, media := range mediaCases {
					cases = append(cases, compressionTestCase{
						uncompressedSize: uncompressedSize,
						uncompressedBufferSize: uncompressedBufferSize,
						compressedBufferSize: compressedBufferSize,
						media: media,
					})
				}
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
		if words.Len() + len(word) > size {
			// pad words to exact length
			words.Write(make([]byte, size - words.Len()))
			break
		}
		words.WriteString(word)
	}

	return words
}


