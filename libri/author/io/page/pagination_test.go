package page

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/common"
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

type fixedEncrypter struct {
	encryptBytes []byte
	encryptErr   error
}

func (f *fixedEncrypter) Encrypt(plaintext []byte, pageIndex uint32) ([]byte, error) {
	return f.encryptBytes, f.encryptErr
}

func TestPaginator_ReadFrom_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	keys, _, _ := enc.NewPseudoRandomKeys(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)
	pages := make(chan *api.Page, 3)
	compressedBytes := []byte("some fake compressed bytes")

	encrypter, err := enc.NewEncrypter(keys)
	assert.Nil(t, err)

	// check that compressed read error bubbles up
	p, err := NewPaginator(pages, encrypter, keys, authorPub, MinSize)
	assert.Nil(t, err)
	n, err := p.ReadFrom(errReader{})
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that ciphertextMAC.Write(...) error bubbles up
	p, err = NewPaginator(pages, &fixedEncrypter{}, keys, authorPub, MinSize)
	assert.Nil(t, err)
	_, err = p.ReadFrom(bytes.NewReader(compressedBytes))
	assert.NotNil(t, err)

	// check that encyption error bubbles up
	encrypter = &fixedEncrypter{encryptErr: errors.New("some Encrypt error")}
	p, err = NewPaginator(pages, encrypter, keys, authorPub, MinSize)
	assert.Nil(t, err)
	_, err = p.ReadFrom(bytes.NewReader(compressedBytes))
	assert.NotNil(t, err)

	// check getPage(...) error bubbles up
	encrypter = &fixedEncrypter{encryptBytes: []byte("some ciphertext bytes")}
	p, err = NewPaginator(pages, encrypter, keys, authorPub, MinSize)
	assert.Nil(t, err)
	p.(*paginator).authorPub = nil // will cause ValidatePage error in getPage(...)
	_, err = p.ReadFrom(bytes.NewReader(compressedBytes))
	assert.NotNil(t, err)
}

func TestPaginator_GetPage_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	// check error from pageMAC.Write(...) bubbles up
	p1 := &paginator{pageMAC: enc.NewHMAC([]byte("HMAC key"))}
	page, err := p1.getPage(nil, 0)
	assert.NotNil(t, err)
	assert.Nil(t, page)

	// check error from api.ValidatePage(...) bubbles up
	p2 := &paginator{
		pageMAC:   enc.NewHMAC([]byte("HMAC key")),
		authorPub: nil, // will cause invalidate page to be created
	}
	page, err = p2.getPage(api.RandBytes(rng, 64), 0)
	assert.NotNil(t, err)
	assert.Nil(t, page)
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
	err            error
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

	// check that invalid page creates an error
	u1, err := NewUnpaginator(pages, decrypter, keys)
	assert.Nil(t, err)
	pages <- &api.Page{} // invalid page
	n, err := u1.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that out of order page creates an error
	ciphertext2 := []byte("some secret stuff")
	ciphertextMac2 := enc.HMAC(ciphertext2, keys.HMACKey)
	pages <- &api.Page{
		AuthorPublicKey: api.RandBytes(rng, api.ECPubKeyLength),
		Index:           1, // should start with 0
		Ciphertext:      ciphertext2,
		CiphertextMac:   ciphertextMac2,
	}
	u2, err := NewUnpaginator(pages, decrypter, keys)
	assert.Nil(t, err)

	n, err = u2.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that bad ciphertext MAC creates error
	u3, err := NewUnpaginator(pages, decrypter, keys)
	assert.Nil(t, err)
	pages <- &api.Page{
		AuthorPublicKey: api.RandBytes(rng, api.ECPubKeyLength),
		Ciphertext:      []byte("some secret stuff"),
		CiphertextMac:   []byte("not the right mac"),
	}
	n, err = u3.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that Decrypt error bubbles up
	decrypter4 := &fixedDecrypter{
		err: errors.New("some Decrypt error"),
	}
	ciphertext4 := []byte("some secret stuff")
	ciphertextMac4 := enc.HMAC(ciphertext4, keys.HMACKey)
	pages <- &api.Page{
		AuthorPublicKey: api.RandBytes(rng, api.ECPubKeyLength),
		Ciphertext:      ciphertext4,
		CiphertextMac:   ciphertextMac4,
	}
	u4, err := NewUnpaginator(pages, decrypter4, keys)
	assert.Nil(t, err)
	n, err = u4.WriteTo(nil)
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that decompressor write error bubbles up
	decrypter5 := &fixedDecrypter{}
	ciphertext5 := []byte("some other secret stuff")
	ciphertextMac5 := enc.HMAC(ciphertext5, keys.HMACKey)
	pages <- &api.Page{
		AuthorPublicKey: api.RandBytes(rng, api.ECPubKeyLength),
		Ciphertext:      ciphertext5,
		CiphertextMac:   ciphertextMac5,
	}
	u5, err := NewUnpaginator(pages, decrypter5, keys)
	assert.Nil(t, err)
	n, err = u5.WriteTo(&errCloseWriter{
		writeErr: errors.New("some write error"),
	})
	assert.NotNil(t, err)
	assert.Zero(t, n)

	// check that decompressor close error bubbles up
	decrypter6 := &fixedDecrypter{}
	ciphertext6 := []byte("some other other secret stuff")
	ciphertextMac6 := enc.HMAC(ciphertext6, keys.HMACKey)
	pages <- &api.Page{
		AuthorPublicKey: api.RandBytes(rng, api.ECPubKeyLength),
		Ciphertext:      ciphertext6,
		CiphertextMac:   ciphertextMac6,
	}
	close(pages)
	u6, err := NewUnpaginator(pages, decrypter6, keys)
	assert.Nil(t, err)
	n, err = u6.WriteTo(&errCloseWriter{
		closeErr: errors.New("some close error"),
	})
	assert.NotNil(t, err)
	assert.Zero(t, n)
}

func TestCheckCiphertextMAC_err(t *testing.T) {
	u := &unpaginator{pageMAC: enc.NewHMAC([]byte("HMAC key"))}

	// check pageMAC.Write(...) error bubbles up
	err := u.checkCiphertextMAC(&api.Page{})
	assert.NotNil(t, err)

	// check not equal MACs returns error
	err = u.checkCiphertextMAC(&api.Page{Ciphertext: []byte("some secret")})
	assert.Equal(t, ErrUnexpectedCiphertextMAC, err)
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

		uncompressed1 := common.NewCompressableBytes(rng, c.uncompressedSize)
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
	return fmt.Sprintf("pageSize: %d, uncompressedSize: %d, codec: %s", p.pageSize,
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
