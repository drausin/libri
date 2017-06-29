package page

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/drausin/libri/libri/author/io/comp"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/librarian/api"
)

var (
	// MinSize is the smallest maximum number of bytes in a page.
	MinSize = uint32(64 * 1024) // 64 MB

	// DefaultSize is the default maximum number of bytes in a page.
	DefaultSize = uint32(2 * 1024 * 1024) // 2 MB
)

// ErrUnexpectedCiphertextMAC indicates when the ciphertext MAC does not match the expected value.
var ErrUnexpectedCiphertextMAC = errors.New("ciphertext mac does not match expected value")

// ErrPageSizeTooSmall indicates when the max page size is too small (often because it is zero).
var ErrPageSizeTooSmall = fmt.Errorf("page size is below %d byte minimum", MinSize)

// Paginator is an io.ReaderFrom that reads from a compressor and writes encrypted pages to a
// channel.
type Paginator interface {
	io.ReaderFrom

	// CiphertextMAC is the MAC for the entire ciphertext across all pages.
	CiphertextMAC() enc.MAC
}

// paginator is an io.ReaderFrom that reads compressed bytes and emits them in discrete pages.
type paginator struct {
	pages         chan *api.Page
	encrypter     enc.Encrypter
	pageSize      uint32
	authorPub     []byte
	pageMAC       enc.MAC
	ciphertextMAC enc.MAC
}

// NewPaginator creates a new paginator that emits pages to the given channel.
func NewPaginator(
	pages chan *api.Page,
	encrypter enc.Encrypter,
	keys *enc.EEK,
	authorPub []byte,
	pageSize uint32,
) (Paginator, error) {
	if err := api.ValidateHMACKey(keys.HMACKey); err != nil {
		return nil, err
	}
	if err := api.ValidatePublicKey(authorPub); err != nil {
		return nil, err
	}
	if pageSize < MinSize {
		return nil, ErrPageSizeTooSmall
	}
	return &paginator{
		pages:         pages,
		encrypter:     encrypter,
		pageSize:      pageSize,
		authorPub:     authorPub,
		pageMAC:       enc.NewHMAC(keys.HMACKey),
		ciphertextMAC: enc.NewHMAC(keys.HMACKey),
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

		if _, err = p.ciphertextMAC.Write(pageCiphertext); err != nil {
			return n, err
		}

		page, err := p.getPage(pageCiphertext, i)
		if err != nil {
			return n, err
		}
		p.pages <- page
	}
	return n, nil
}

// getPage constructs a page from a given ciphertext.
func (p *paginator) getPage(ciphertext []byte, index uint32) (*api.Page, error) {
	p.pageMAC.Reset()
	if _, err := p.pageMAC.Write(ciphertext); err != nil {
		return nil, err
	}
	page := &api.Page{
		AuthorPublicKey: p.authorPub,
		Index:           index,
		Ciphertext:      ciphertext,
		CiphertextMac:   p.pageMAC.Sum(nil),
	}
	if err := api.ValidatePage(page); err != nil {
		// extra safeguard
		return nil, err
	}
	return page, nil
}

func (p *paginator) CiphertextMAC() enc.MAC {
	return p.ciphertextMAC
}

// Unpaginator writes content from discrete pages to a decompressed writer.
type Unpaginator interface {
	// WriteTo writes content from the underlying channel of pages to the decompressor.
	WriteTo(decompressor comp.CloseWriter) (int64, error)

	// CiphertextMAC is the MAC for the entire ciphertext across all pages.
	CiphertextMAC() enc.MAC
}

type unpaginator struct {
	pages         chan *api.Page
	decrypter     enc.Decrypter
	compressedBuf *bytes.Buffer
	pageMAC       enc.MAC
	ciphertextMAC enc.MAC
}

// NewUnpaginator creates a new Unpaginator from the channel of pages and decrypter.
func NewUnpaginator(
	pages chan *api.Page,
	decrypter enc.Decrypter,
	keys *enc.EEK,
) (Unpaginator, error) {
	if err := api.ValidateHMACKey(keys.HMACKey); err != nil {
		return nil, err
	}
	return &unpaginator{
		pages:         pages,
		decrypter:     decrypter,
		pageMAC:       enc.NewHMAC(keys.HMACKey),
		ciphertextMAC: enc.NewHMAC(keys.HMACKey),
	}, nil
}

func (u *unpaginator) WriteTo(decompressor comp.CloseWriter) (int64, error) {
	var n int64
	var pageIndex uint32
	for page := range u.pages {
		if err := api.ValidatePage(page); err != nil {
			return n, err
		}
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
	if _, err := u.pageMAC.Write(page.Ciphertext); err != nil {
		return err
	}
	if !bytes.Equal(u.pageMAC.Sum(nil), page.CiphertextMac) {
		return ErrUnexpectedCiphertextMAC
	}
	return nil
}

func (u *unpaginator) CiphertextMAC() enc.MAC {
	return u.ciphertextMAC
}
