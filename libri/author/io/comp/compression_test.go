package comp

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/pkg/errors"
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

func TestNewCompressor_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	uncompressed, codec, minUncompressedBufferSize := new(bytes.Buffer), GZIPCodec, uint32(256)
	comp, err := NewCompressor(uncompressed, codec, keys, minUncompressedBufferSize)
	assert.Nil(t, err)
	assert.Equal(t, uncompressed, comp.(*compressor).uncompressed)
	assert.NotNil(t, comp.(*compressor).inner)
	assert.NotNil(t, comp.(*compressor).buf)
	assert.Equal(t, minUncompressedBufferSize, comp.(*compressor).uncompressedBufferSize)
}

func TestNewCompressor_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)

	// unexpected codec
	assert.Panics(t, func() {
		_, err := NewCompressor(
			new(bytes.Buffer),
			Codec("unexpected"),
			keys,
			MinBufferSize,
		)
		assert.Nil(t, err)
	})

	// too small uncompressed buffer
	comp, err := NewCompressor(new(bytes.Buffer), GZIPCodec, keys, 0)
	assert.NotNil(t, err)
	assert.Nil(t, comp)
}

func TestNewDecompressor_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	uncompressed, codec, minUncompressedBufferSize := new(bytes.Buffer), GZIPCodec, uint32(256)
	comp, err := NewDecompressor(uncompressed, codec, keys, minUncompressedBufferSize)
	assert.Nil(t, err)
	assert.Equal(t, uncompressed, comp.(*decompressor).uncompressed)
	assert.Nil(t, comp.(*decompressor).inner)
	assert.NotNil(t, comp.(*decompressor).buf)
	assert.Equal(t, minUncompressedBufferSize, comp.(*decompressor).uncompressedBufferSize)
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("some read error")
}

type errFlushCloseWriter struct {
	writeErr error
	flushErr error
	closeErr error
}

func (w errFlushCloseWriter) Write(p []byte) (int, error) {
	return 0, w.writeErr
}

func (w errFlushCloseWriter) Flush() error {
	return w.flushErr
}

func (w errFlushCloseWriter) Close() error {
	return w.closeErr
}

func TestCompressor_Read_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	uncompressed1 := newTestBytes(rng, 256)
	uncompressed1Bytes := uncompressed1.Bytes()

	comp, err := NewCompressor(
		uncompressed1,
		GZIPCodec,
		keys,
		MinBufferSize,
	)
	assert.Nil(t, err)

	compressed := make([]byte, len(uncompressed1Bytes))
	n1, err := comp.Read(compressed)
	assert.Nil(t, err)
	assert.True(t, n1 > 0)

	reader, err := gzip.NewReader(bytes.NewReader(compressed[:n1]))
	assert.Nil(t, err)

	// check Compressor and raw GZIP compressor return same compressed bytes
	uncompressed2 := new(bytes.Buffer)
	n2, err := uncompressed2.ReadFrom(reader)
	assert.Nil(t, err)
	assert.Equal(t, len(uncompressed1Bytes), int(n2))
	assert.Equal(t, uncompressed1Bytes, uncompressed2.Bytes())

	// TODO check uncompressedMAC
}

func TestCompressor_Read_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	comp, err := NewCompressor(errReader{}, GZIPCodec, keys, MinBufferSize)
	assert.Nil(t, err)

	// check that error from errReader bubbles up
	n, err := comp.Read(make([]byte, 32))
	assert.NotNil(t, err)
	assert.Zero(t, n)

	buf := bytes.NewReader([]byte("some data"))
	comp, err = NewCompressor(buf, GZIPCodec, keys, MinBufferSize)
	comp.(*compressor).inner = errFlushCloseWriter{
		writeErr: errors.New("some write error"),
	}
	assert.Nil(t, err)

	// check that write error bubbles up
	n, err = comp.Read(make([]byte, 32))
	assert.NotNil(t, err)
	assert.Zero(t, n)

	buf = bytes.NewReader([]byte("some data"))
	comp, err = NewCompressor(buf, GZIPCodec, keys, MinBufferSize)
	comp.(*compressor).inner = errFlushCloseWriter{
		flushErr: errors.New("some flush error"),
	}
	assert.Nil(t, err)

	// TODO check MAC error bubbles up

	// check that flush error bubbles up
	n, err = comp.Read(make([]byte, 32))
	assert.NotNil(t, err)
	assert.Zero(t, n)

	buf = bytes.NewReader([]byte("some data"))
	comp, err = NewCompressor(buf, GZIPCodec, keys, MinBufferSize)
	comp.(*compressor).inner = errFlushCloseWriter{
		closeErr: errors.New("some close error"),
	}
	assert.Nil(t, err)

	// check that close error bubbles up
	n, err = comp.Read(make([]byte, 32))
	assert.NotNil(t, err)
	assert.Zero(t, n)

	buf = bytes.NewReader([]byte("some data"))
	comp, err = NewCompressor(buf, GZIPCodec, keys, MinBufferSize)
	comp.(*compressor).buf = new(bytes.Buffer) // will case buf.Read() to return io.EOF error
	assert.Nil(t, err)

	// check that buf.Read() error bubbles up
	n, err = comp.Read(make([]byte, 32))
	assert.NotNil(t, err)
	assert.Zero(t, n)
}

func TestDecompressor_Write_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	uncompressed1 := newTestBytes(rng, 256).Bytes()

	compressed := new(bytes.Buffer)
	writer := gzip.NewWriter(compressed)
	n, err := writer.Write(uncompressed1)
	assert.Nil(t, err)
	assert.NotZero(t, n)

	err = writer.Close()
	assert.Nil(t, err)
	assert.NotZero(t, compressed.Len())

	uncompressed2 := new(bytes.Buffer)
	decomp, err := NewDecompressor(uncompressed2, GZIPCodec, keys, MinBufferSize)
	assert.Nil(t, err)

	n, err = decomp.Write(compressed.Bytes())
	assert.Nil(t, err)
	assert.NotZero(t, n)

	err = decomp.Close()
	assert.Nil(t, err)

	assert.Equal(t, uncompressed1, uncompressed2.Bytes())
}

func TestDecompressor_Write_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	decomp, err := NewDecompressor(nil, GZIPCodec, keys, MinBufferSize)
	assert.Nil(t, err)
	decomp.(*decompressor).closed = true
	compressed := make([]byte, MinBufferSize*2)

	// check that Write when decomp is closed produces errors
	n, err := decomp.Write(compressed)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	decomp, err = NewDecompressor(new(bytes.Buffer), GZIPCodec, keys, MinBufferSize)
	assert.Nil(t, err)

	// check that failure to create inner decompressor bubbles up
	n, err = decomp.Write(compressed)
	assert.NotNil(t, err)
	assert.Equal(t, len(compressed), n)

	// TODO check that MAC write error bubbles up

	decomp, err = NewDecompressor(new(bytes.Buffer), GZIPCodec, keys, MinBufferSize)
	assert.Nil(t, err)
	decomp.(*decompressor).inner = errReader{}

	// check that inner.Read() error bubbles up
	n, err = decomp.Write(compressed)
	assert.NotNil(t, err)
	assert.Equal(t, len(compressed), n)

	// check that inner.Close() error bubbles up
	err = decomp.Close()
	assert.NotNil(t, err)
}

func TestCompressDecompress(t *testing.T) {
	mediaCases := []mediaTestCase{
		{GZIPCodec, false},
		{NoneCodec, true}, // equalSize since we're not compressing twice
	}
	uncompressedSizes := []int{128, 192, 256, 384, 512, 1024}
	uncompressedBufferSizes := []uint32{128, 192, 256, 384, 512, 1024}
	compressedBufferSizes := []int{128, 192, 256, 384, 512, 1024}
	cases := caseCrossProduct(
		uncompressedSizes,
		uncompressedBufferSizes,
		compressedBufferSizes,
		mediaCases,
	)

	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	for _, c := range cases {
		uncompressed1 := newTestBytes(rng, c.uncompressedSize)
		uncompressed1Bytes := uncompressed1.Bytes()
		assert.Equal(t, c.uncompressedSize, uncompressed1.Len())

		compressor, err := NewCompressor(uncompressed1, c.media.codec,
			keys, c.uncompressedBufferSize)
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
			// make sure comp.has actually happened
			assert.True(t, compressed.Len() < c.uncompressedSize, c.String())
		}

		uncompressed2 := new(bytes.Buffer)
		decompressor, err := NewDecompressor(uncompressed2, c.media.codec,
			keys, c.uncompressedBufferSize)
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

type compTestCase struct {
	uncompressedSize       int
	uncompressedBufferSize uint32
	compressedBufferSize   int
	media                  mediaTestCase
}

func (c compTestCase) String() string {
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
	uncompressedBufferSizes []uint32,
	compressedBufferSizes []int,
	mediaCases []mediaTestCase,
) []compTestCase {
	cases := make([]compTestCase, 0)
	for _, uncompressedSize := range uncompressedSizes {
		for _, uncompressedBufferSize := range uncompressedBufferSizes {
			for _, compressedBufferSize := range compressedBufferSizes {
				for _, media := range mediaCases {
					cases = append(cases, compTestCase{
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
