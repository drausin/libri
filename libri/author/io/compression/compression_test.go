package compression

import (
	"testing"
	"math/rand"
	"github.com/stretchr/testify/assert"
	"bytes"
	"fmt"
	"io"
)

func TestNewCompressorDecompressor(t *testing.T) {
	cases := []struct {
		mediaType string
		equalSize bool
	}{
		{"application/pdf", false},
		{"application/x-gzip", true},  // equalSize since we're not compressing twice
	}

	rng := rand.New(rand.NewSource(0))
	uncompressed1 := newTestBytes(rng, 32)
	for _, c := range cases {
		info := fmt.Sprintf("mediaType: %s", c.mediaType)

		compressed := new(bytes.Buffer)
		compressor, err := NewCompressor(compressed, c.mediaType)
		assert.Nil(t, err, info)

		n1, err := compressor.Write(uncompressed1)
		assert.Nil(t, err, info)
		assert.Equal(t, n1, len(uncompressed1), info)

		assert.Nil(t, compressor.Close())
		assert.True(t, c.equalSize || compressed.Len() < len(uncompressed1), info)

		decompressor, err := NewDecompressor(compressed, c.mediaType)
		assert.Nil(t, err, info)

		uncompressed2 := new(bytes.Buffer)
		_, err = io.Copy(uncompressed2, decompressor)
		assert.Nil(t, err)
		assert.Equal(t, uncompressed1, uncompressed2.Bytes(), info)

		assert.Nil(t, decompressor.Close())
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

func newTestBytes(rng *rand.Rand, nWords int) []byte {
	dict := []string{
		"these", "are", "some", "test", "words", "that", "will", "be", "compressed",
	}
	words := ""
	for c := 0; c < nWords; c++ {
		words += dict[int(rng.Int31n(int32(len(dict))))] + " "
	}

	return []byte(words)
}