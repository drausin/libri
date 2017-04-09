package page

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
)

func TestNewPaginator_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys1, authorPub1, _ := enc.NewPseudoRandomKeys(rng)
	keys1.HMACKey = nil

	// invalid HMACKey should bubble up
	p1, err := NewPaginator(nil, nil, keys1, authorPub1, MinSize)
	assert.NotNil(t, err)
	assert.Nil(t, p1)

	keys2, _, _ := enc.NewPseudoRandomKeys(rng)

	// invalid author public key should bubble up
	p2, err := NewPaginator(nil, nil, keys2, nil, MinSize)
	assert.NotNil(t, err)
	assert.Nil(t, p2)

	keys3, authorPub3, _ := enc.NewPseudoRandomKeys(rng)

	// too small page size should create error
	p3, err := NewPaginator(nil, nil, keys3, authorPub3, 0)
	assert.NotNil(t, err)
	assert.Nil(t, p3)
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("some read error")
}

type errEncrypter struct{}

func (errEncrypter) Encrypt(plaintext []byte, pageIndex uint32) ([]byte, error) {
	return nil, errors.New("some encrypt error")
}

func TestPaginator_ReadFrom_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)
	pages := make(chan *api.Page, 3)

	encrypter, err := enc.NewEncrypter(keys)
	assert.Nil(t, err)
	paginator, err := NewPaginator(pages, encrypter, keys, authorPub, MinSize)
	assert.Nil(t, err)

	// check that compressed read error bubbles up
	n, err := paginator.ReadFrom(errReader{})
	assert.NotNil(t, err)
	assert.Zero(t, n)

	paginator, err = NewPaginator(pages, errEncrypter{}, keys, authorPub, MinSize)
	assert.Nil(t, err)

	// check that encyption error bubbles up
	_, err = paginator.ReadFrom(bytes.NewReader([]byte("some fake compressed bytes")))
	assert.NotNil(t, err)
}

func TestNewUnpaginator(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
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

type fixedDecrypter struct {
	compressedPage []byte
	err error
}

func (f *fixedDecrypter) Decrypt(ciphertext []byte, pageIndex uint32) ([]byte, error) {
	return f.compressedPage, f.err
}

func TestUnpaginator_WriteTo_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	decrypter, err := enc.NewDecrypter(keys)
	assert.Nil(t, err)
	pages := make(chan *api.Page, 1)

	u1, err := NewUnpaginator(pages, decrypter, keys)
	assert.Nil(t, err)

	// check that out of order page creates an error
	pages <- &api.Page{Index: 1} // should start with 0
	n, err := u1.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	u2, err := NewUnpaginator(pages, decrypter, keys)
	assert.Nil(t, err)

	// check that bad ciphertext MAC creates error
	pages <- &api.Page{
		Ciphertext:    []byte("some secret stuff"),
		CiphertextMac: []byte("not the right mac"),
	}
	n, err = u2.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)


	decrypter3 := &fixedDecrypter{
		err: errors.New("some Decrypt error"),
	}
	ciphertext3 := []byte("some secret stuff")
	ciphertextMac3, err := enc.HMAC(ciphertext3, keys.HMACKey)
	assert.Nil(t, err)
	pages <- &api.Page{
		Ciphertext: ciphertext3,
		CiphertextMac: ciphertextMac3,
	}
	u3, err := NewUnpaginator(pages, decrypter3, keys)
	assert.Nil(t, err)

	// check that Decrypt error bubbles up
	n, err = u3.WriteTo(nil)


	decrypter4 := &fixedDecrypter{}
	ciphertext4 := []byte("some other secret stuff")
	ciphertextMac4, err := enc.HMAC(ciphertext4, keys.HMACKey)
	assert.Nil(t, err)
	pages <- &api.Page{
		Ciphertext: ciphertext4,
		CiphertextMac: ciphertextMac4,
	}
	u4, err := NewUnpaginator(pages, decrypter4, keys)
	assert.Nil(t, err)

	// check that decompressor write error bubbles up
	n, err = u4.WriteTo(&errCloseWriter{
		writeErr: errors.New("some write error"),
	})
	assert.NotNil(t, err)
	assert.Zero(t, n)


	decrypter5 := &fixedDecrypter{}
	ciphertext5 := []byte("some other other secret stuff")
	ciphertextMac5, err := enc.HMAC(ciphertext5, keys.HMACKey)
	assert.Nil(t, err)
	pages <- &api.Page{
		Ciphertext: ciphertext5,
		CiphertextMac: ciphertextMac5,
	}
	u5, err := NewUnpaginator(pages, decrypter5, keys)
	assert.Nil(t, err)

	// check that decompressor close error bubbles up
	n, err = u5.WriteTo(&errCloseWriter{
		closeErr: errors.New("some close error"),
	})
	assert.NotNil(t, err)
	assert.Zero(t, n)
}

func TestPaginateUnpaginate(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)

	encrypter, err := enc.NewEncrypter(keys)
	assert.Nil(t, err)
	decrypter, err := enc.NewDecrypter(keys)
	assert.Nil(t, err)

	MinSize = 64 // just for testing
	uncompressedSizes := []int{128, 192, 256, 384, 512, 768, 1024, 2048, 4096, 8192}
	pageSizes := []uint32{128, 256, 512, 1024}
	codecs := []comp.Codec{comp.GZIPCodec, comp.NoneCodec}

	for _, c := range caseCrossProduct(pageSizes, uncompressedSizes, codecs) {
		pages := make(chan *api.Page, 3)
		paginator, err := NewPaginator(pages, encrypter, keys, authorPub, c.pageSize)
		assert.Nil(t, err)

		uncompressed1 := newTestBytes(rng, c.uncompressedSize)
		uncompressed1Bytes := uncompressed1.Bytes()

		uncompressedBufferSize := uint32(c.pageSize) / 2
		compressor, err := comp.NewCompressor(uncompressed1, c.codec, keys,
			uncompressedBufferSize)
		assert.Nil(t, err)
		assert.NotNil(t, compressor)

		uncompressed2 := new(bytes.Buffer)
		decompressor, err := comp.NewDecompressor(uncompressed2, c.codec, keys,
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
		assert.Equal(t, paginator.CiphertextMAC().MessageSize(),
			unpaginator.CiphertextMAC().MessageSize())
		assert.Equal(t, paginator.CiphertextMAC().Sum(nil),
			unpaginator.CiphertextMAC().Sum(nil))
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
