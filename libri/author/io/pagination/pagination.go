package pagination

import (
	"io"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/author/io/encryption"
	"github.com/drausin/libri/libri/common/ecid"
	"hash"
	"crypto/hmac"
	"crypto/sha256"
	"github.com/pkg/errors"
	"fmt"
	"bytes"
	"github.com/drausin/libri/libri/author/io/compression"
)


type paginator struct {
	pages     chan *api.Page
	encrypter encryption.Encrypter
	pageSize  uint32
	authorID ecid.ID
	pageMACer hash.Hash
}

func NewPaginator(
	pages chan *api.Page,
	encrypter encryption.Encrypter,
	authorID ecid.ID,
	pageHMACKey []byte,
	pageSize uint32,
) (io.ReaderFrom, error) {
	if err := api.ValidatePageHMACKey(pageHMACKey); err != nil {
		return nil, err
	}
	return &paginator{
		pages: pages,
		encrypter: encrypter,
		pageSize: pageSize,
		authorID: authorID,
		pageMACer: hmac.New(sha256.New, pageHMACKey),
	}, nil
}

func (p *paginator) ReadFrom(compressor io.Reader) (int64, error) {
	var n int64
	var ni int
	var err error
	compressedPage := make([]byte, int(p.pageSize))
	for i := uint32(0); n == 0 || uint32(ni) == p.pageSize; i++ {

		// read a page of compressed contents
		ni, err = compressor.Read(compressedPage)
		n += int64(ni)
		if err != nil && err != io.EOF {
			return n, err
		}

		// encrypt compressed page
		pageCiphertext, err := p.encrypter.Encrypt(compressedPage[:ni], i)
		if err != nil {
			return n, err
		}

		p.pages <- p.getPage(pageCiphertext, i)
	}
	return n, nil
}

func (p *paginator) getPage(ciphertext []byte, index uint32) *api.Page {
	p.pageMACer.Reset()
	return &api.Page{
		AuthorPublicKey: p.authorID.Bytes(),
		Index: index,
		Ciphertext: ciphertext,
		CiphertextMac: p.pageMACer.Sum(ciphertext),
	}
}

type unpaginator struct {
	pages chan *api.Page
	decrypter encryption.Decrypter
	compressedBuf *bytes.Buffer
	pageMACer hash.Hash
}

type Unpaginator interface {
	WriteTo(decompressor compression.FlushWriter) (int64, error)
}

func NewUnpaginator(
	pages chan *api.Page,
	decrypter encryption.Decrypter,
	pageHMACKey []byte,
) (Unpaginator, error) {
	if err := api.ValidatePageHMACKey(pageHMACKey); err != nil {
		return nil, err
	}
	return &unpaginator{
		pages: pages,
		decrypter: decrypter,
		pageMACer: hmac.New(sha256.New, pageHMACKey),
	}, nil
}

func (u *unpaginator) WriteTo(decompressor compression.FlushWriter) (int64, error) {
	var n int64
	var pageIndex uint32
	for page := range u.pages {
		if page.Index != pageIndex {
			return n, fmt.Errorf("received out of order page index %d, expected %d",
				page.Index, pageIndex)
		}
		if err := u.checkCiphertextMAC(page); err != nil {
			return n, err
		}

		compressedPage, err := u.decrypter.Decrypt(page.Ciphertext, page.Index)
		if err != nil {
			return n, err
		}

		np, err := decompressor.Write(compressedPage)
		if err != nil {
			return n, err
		}
		n += int64(np)
		pageIndex++
	}

	return n, decompressor.Flush()
}

func (u *unpaginator) checkCiphertextMAC(page *api.Page) error {
	u.pageMACer.Reset()
	mac := u.pageMACer.Sum(page.Ciphertext)
	if !bytes.Equal(mac, page.CiphertextMac) {
		return errors.New("calculated ciphertext mac does not match supplied value")
	}
	return nil
}
