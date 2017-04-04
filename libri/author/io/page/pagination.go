package page

import (
	"bytes"
	"fmt"
	"io"
	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"github.com/drausin/libri/libri/author/io/encryption"
)

const (
	DefaultSize = uint32(2 * 1024 * 1024) // 2 MB
)

type Paginator interface {
	io.ReaderFrom

	// CiphertextMAC is the MAC for the entire ciphertext across all pages.
	CiphertextMAC() encryption.MAC
}

// paginator is an io.ReaderFrom that reads compressed bytes and emits them in discrete pages.
type paginator struct {
	pages         chan *api.Page
	encrypter     enc.Encrypter
	pageSize      uint32
	authorID      ecid.ID
	pageMAC       encryption.MAC
	ciphertextMAC encryption.MAC
}

// NewPaginator creates a new paginator that emits pages to the given channel.
func NewPaginator(
	pages chan *api.Page,
	encrypter enc.Encrypter,
	keys *enc.Keys,
	authorID ecid.ID,
	pageSize uint32,
) (Paginator, error) {
	if err := api.ValidateHMACKey(keys.HMACKey); err != nil {
		return nil, err
	}
	return &paginator{
		pages:         pages,
		encrypter:     encrypter,
		pageSize:      pageSize,
		authorID:      authorID,
		pageMAC:       encryption.NewHMAC(keys.HMACKey),
		ciphertextMAC: encryption.NewHMAC(keys.HMACKey),
	}, nil
}

// ReadFrom reads pages from the compressor io.Reader and emits encrypted pages to the
// underlying channel.
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
		if _, err := p.ciphertextMAC.Write(pageCiphertext); err != nil {
			return n, err
		}

		p.pages <- p.getPage(pageCiphertext, i)
	}
	return n, nil
}

// getPage constructs a page from a given ciphertext.
func (p *paginator) getPage(ciphertext []byte, index uint32) *api.Page {
	p.pageMAC.Reset()
	return &api.Page{
		AuthorPublicKey: ecid.ToPublicKeyBytes(p.authorID),
		Index:           index,
		Ciphertext:      ciphertext,
		CiphertextMac:   p.pageMAC.Sum(ciphertext),
	}
}

func (p *paginator) CiphertextMAC() encryption.MAC {
	return p.ciphertextMAC
}

// Unpaginator writes content from discrete pages to a decompressed writer.
type Unpaginator interface {
	// WriteTo writes content from the underlying channel of pages to the decompressor.
	WriteTo(decompressor comp.CloseWriter) (int64, error)

	// CiphertextMAC is the MAC for the entire ciphertext across all pages.
	CiphertextMAC() encryption.MAC
}

type unpaginator struct {
	pages         chan *api.Page
	decrypter     enc.Decrypter
	compressedBuf *bytes.Buffer
	pageMAC       encryption.MAC
	ciphertextMAC encryption.MAC
}

// NewUnpaginator creates a new Unpaginator from the channel of pages and decrypter.
func NewUnpaginator(
	pages chan *api.Page,
	decrypter enc.Decrypter,
	keys *enc.Keys,
) (Unpaginator, error) {
	if err := api.ValidateHMACKey(keys.HMACKey); err != nil {
		return nil, err
	}
	return &unpaginator{
		pages:     pages,
		decrypter: decrypter,
		pageMAC: encryption.NewHMAC(keys.HMACKey),
		ciphertextMAC: encryption.NewHMAC(keys.HMACKey),
	}, nil
}

func (u *unpaginator) WriteTo(decompressor comp.CloseWriter) (int64, error) {
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
		if _, err := u.ciphertextMAC.Write(page.Ciphertext); err != nil {
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

	return n, decompressor.Close()
}

// checkCiphertextMac checks that a given page's message authentication code (MAC) matches the
// supplied value.
func (u *unpaginator) checkCiphertextMAC(page *api.Page) error {
	u.pageMAC.Reset()
	mac := u.pageMAC.Sum(page.Ciphertext)
	if !bytes.Equal(mac, page.CiphertextMac) {
		return errors.New("calculated ciphertext mac does not match supplied value")
	}
	return nil
}

func (u *unpaginator) CiphertextMAC() encryption.MAC {
	return u.ciphertextMAC
}

