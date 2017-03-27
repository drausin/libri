package compression

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestCompressDecompress(t *testing.T) {
	mediaCases := []mediaTestCase{
		{GZIPCodec, false},
		{NoneCodec, true}, // equalSize since we're not compressing twice
	}
	uncompressedSizes := []int{128, 192, 256, 384, 512, 1024}
	uncompressedBufferSizes := []int{128, 192, 256, 384, 512, 1024}
	compressedBufferSizes := []int{128, 192, 256, 384, 512, 1024}
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

		compressor, err := NewCompressor(uncompressed1, c.media.codec,
			c.uncompressedBufferSize)
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
		decompressor, err := NewDecompressor(uncompressed2, c.media.codec,
			c.uncompressedBufferSize)
		assert.Nil(t, err, c.String())

		compressedLen := compressed.Len()
		n2, err := decompressor.Write(compressed.Bytes())
		assert.Equal(t, compressedLen, n2)
		assert.Nil(t, err)

		err = decompressor.Close()
		assert.Nil(t, err)

		assert.Equal(t, len(uncompressed1Bytes), uncompressed2.Len(), c.String())
		assert.Equal(t, uncompressed1Bytes, uncompressed2.Bytes(), c.String())
	}
}

func TestTrimBuffer(t *testing.T) {
	buf := new(bytes.Buffer)

	originalMessage := bytes.Repeat([]byte{1, 2, 3, 4}, 4)
	buf.Write(originalMessage)

	part1 := make([]byte, len(originalMessage)/2)
	n1, err := buf.Read(part1)
	assert.Nil(t, err)
	assert.Equal(t, n1, len(part1))

	err = trimBuffer(buf)
	assert.Nil(t, err)

	part2 := make([]byte, len(originalMessage)/2)
	n2, err := buf.Read(part2)
	assert.Nil(t, err)
	assert.Equal(t, n2, len(part2))

	readMessage := append(append([]byte{}, part1...), part2...)
	assert.Equal(t, originalMessage, readMessage)
}

type mediaTestCase struct {
	codec     Codec
	equalSize bool
}

type compressionTestCase struct {
	uncompressedSize       int
	uncompressedBufferSize int
	compressedBufferSize   int
	media                  mediaTestCase
}

func (c compressionTestCase) String() string {
	return fmt.Sprintf(
		"uncompressedSize: %d, uncompressedBufferSize: %d, "+
			"compressedBufferSize: %d, codec: %v, equalSize: %v",
		c.uncompressedSize,
		c.uncompressedBufferSize,
		c.compressedBufferSize,
		c.media.codec,
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
						uncompressedSize:       uncompressedSize,
						uncompressedBufferSize: uncompressedBufferSize,
						compressedBufferSize:   compressedBufferSize,
						media:                  media,
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
		if words.Len()+len(word) > size {
			// pad words to exact length
			words.Write(make([]byte, size-words.Len()))
			break
		}
		words.WriteString(word)
	}

	return words
}
