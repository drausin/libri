package page

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
	"errors"
	"github.com/drausin/libri/libri/author/io/comp"
)

func TestNewPaginator_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys1 := enc.NewPseudoRandomKeys(rng)
	keys1.HMACKey = nil

	// invalid HMACKey should bubble up
	p1, err := NewPaginator(nil, nil, keys1, api.RandBytes(rng, api.ECPubKeyLength), 0)
	assert.NotNil(t, err)
	assert.Nil(t, p1)

	keys2 := enc.NewPseudoRandomKeys(rng)

	// invalid author public key should bubble up
	p2, err := NewPaginator(nil, nil, keys2, nil, 0)
	assert.NotNil(t, err)
	assert.Nil(t, p2)
}

type errReader struct {}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("some read error")
}

type errEncrypter struct {}

func (errEncrypter) Encrypt(plaintext []byte, pageIndex uint32) ([]byte, error) {
	return nil, errors.New("some encrypt error")
}

func TestPaginator_ReadFrom_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomKeys(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)
	pages := make(chan *api.Page, 3)

	encrypter, err := enc.NewEncrypter(keys)
	assert.Nil(t, err)
	paginator, err := NewPaginator(pages, encrypter, keys, authorPub, 256)
	assert.Nil(t, err)

	// check that compressed read error bubbles up
	n, err := paginator.ReadFrom(errReader{})
	assert.NotNil(t, err)
	assert.Zero(t, n)

	paginator, err = NewPaginator(pages, errEncrypter{}, keys, authorPub, 256)
	assert.Nil(t, err)

	// check that encyption error bubbles up
	_, err = paginator.ReadFrom(bytes.NewReader([]byte("some fake compressed bytes")))
	assert.NotNil(t, err)
}

func TestNewUnpaginator(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomKeys(rng)
	keys.HMACKey = nil

	// invalid HMACKey should bubble up
	u, err := NewUnpaginator(nil, nil, keys)
	assert.NotNil(t, err)
	assert.Nil(t, u)
}

type errCloseWriter struct {
	writeErr error
	closeErr error
}

func (e *errCloseWriter) Write(p []byte) (int, error) {
	return 0, e.writeErr
}

func (e *errCloseWriter) Close() error {
	return e.closeErr
}

func TestUnpaginator_WriteTo_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomKeys(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)
	decrypter, err := enc.NewDecrypter(keys)
	assert.Nil(t, err)
	pages := make(chan *api.Page, 1)
	u, err := NewUnpaginator(pages, decrypter, keys)
	assert.Nil(t, err)

	// check that out of order page creates an error
	pages <- &api.Page{Index: 1} // should start with 0
	n, err := u.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that bad ciphertext MAC creates error
	pages <- &api.Page{
		Ciphertext: []byte("some secret stuff"),
		CiphertextMac: []byte("not the right mac"),
	}
	n, err = u.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// create paginator for just making new pages
	encrypter, err := enc.NewEncrypter(keys)
	assert.Nil(t, err)
	p, err := NewPaginator(pages, encrypter, keys, authorPub, 256)
	assert.Nil(t, err)

	// check that decompressor write error bubbles up
	pages <- p.(*paginator).getPage([]byte("some fake ciphertext"), 0)
	n, err = u.WriteTo(&errCloseWriter{
		writeErr: errors.New("some write error"),
	})
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that decompressor close error bubbles up
	pages <- p.(*paginator).getPage([]byte("some fake ciphertext"), 0)
	n, err = u.WriteTo(&errCloseWriter{
		closeErr: errors.New("some close error"),
	})
	assert.NotNil(t, err)
	assert.Zero(t, n)
}

func TestPaginateUnpaginate(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomKeys(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)

	encrypter, err := enc.NewEncrypter(keys)
	assert.Nil(t, err)
	decrypter, err := enc.NewDecrypter(keys)
	assert.Nil(t, err)

	uncompressedSizes := []int{128, 192, 256, 384, 512, 768, 1024, 2048, 4096, 8192}
	pageSizes := []uint32{128, 256, 512, 1024}
	codecs := []comp.Codec{comp.GZIPCodec, comp.NoneCodec}

	for _, c := range caseCrossProduct(pageSizes, uncompressedSizes, codecs) {
		pages := make(chan *api.Page, 3)
		paginator, err := NewPaginator(pages, encrypter, keys, authorPub, c.pageSize)
		assert.Nil(t, err)

		uncompressed1 := newTestBytes(rng, c.uncompressedSize)
		uncompressed1Bytes := uncompressed1.Bytes()

		uncompressedBufferSize := int(c.pageSize) / 2
		compressor, err := comp.NewCompressor(uncompressed1, c.codec,
			uncompressedBufferSize)
		assert.Nil(t, err)
		assert.NotNil(t, compressor)

		uncompressed2 := new(bytes.Buffer)
		decompressor, err := comp.NewDecompressor(uncompressed2, c.codec,
			uncompressedBufferSize)
		assert.Nil(t, err)

		unpaginator, err := NewUnpaginator(pages, decrypter, keys)
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
	pageSize         uint32
	uncompressedSize int
	codec            comp.Codec
}

func (p pageTestCase) String() string {
	return fmt.Sprintf("pageSize: %d, uncompressedSize: %d, mediaType: %s", p.pageSize,
		p.uncompressedSize, p.codec)
}

func caseCrossProduct(
	pageSizes []uint32, uncompressedSizes []int, codecs []comp.Codec,
) []*pageTestCase {
	cases := make([]*pageTestCase, 0)
	for _, pageSize := range pageSizes {
		for _, uncompressedSize := range uncompressedSizes {
			for _, mediaType := range codecs {
				cases = append(cases, &pageTestCase{
					pageSize:         pageSize,
					uncompressedSize: uncompressedSize,
					codec:            mediaType,
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
