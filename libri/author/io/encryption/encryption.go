package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
)

// TODO after putting compression and encryption together
// - do things need to be closeable, or can we just wrap and flush compression
// - do we need SetPageIndex at all, or can we just put into New...

type Encrypter interface {
	io.WriteCloser
	SetPageIndex(page uint32)
}

type encrypter struct {
	gcmCipher        cipher.AEAD
	pageIVMac        hash.Hash
	currentPageIV    []byte
	output           io.WriteCloser
}

func NewEncrypter(output io.WriteCloser, keys *Keys) (Encrypter, error) {
	block, err := aes.NewCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	gcmCipher, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pageIVMac := hmac.New(sha256.New, keys.PageIVSeed)
	currentPageIV := generatePageIV(uint32(0), pageIVMac, gcmCipher.NonceSize())
	return &encrypter{
		gcmCipher:        gcmCipher,
		pageIVMac:        pageIVMac,
		currentPageIV:    currentPageIV,
		output:           output,
	}, nil
}

func (e *encrypter) SetPageIndex(page uint32) {
	e.currentPageIV = generatePageIV(page, e.pageIVMac, e.gcmCipher.NonceSize())
}

func (e *encrypter) Write(plaintext []byte) (int, error) {
	ciphertext := e.gcmCipher.Seal(nil, e.currentPageIV, plaintext, nil)
	return e.output.Write(ciphertext)
}

func (e *encrypter) Close() error {
	return e.output.Close()
}

type Decrypter interface {
	io.ReadCloser
	SetPageIndex(page uint32)
}

type decrypter struct {
	gcmCipher        cipher.AEAD
	pageIVMac        hash.Hash
	currentPageIV    []byte
	input            io.ReadCloser
}

func NewDecrypter(input io.ReadCloser, keys *Keys) (Decrypter, error) {
	block, err := aes.NewCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	gcmCipher, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	pageIVMac := hmac.New(sha256.New, keys.PageIVSeed)
	currentPageIV := generatePageIV(uint32(0), pageIVMac, gcmCipher.NonceSize())
	return &decrypter{
		gcmCipher:        gcmCipher,
		pageIVMac:        pageIVMac,
		currentPageIV:    currentPageIV,
		input:            input,
	}, nil
}

func (d *decrypter) SetPageIndex(page uint32) {
	d.currentPageIV = generatePageIV(page, d.pageIVMac, d.gcmCipher.NonceSize())
}

func (d *decrypter) Read(plaintext []byte) (int, error) {
	ciphertext := make([]byte, len(plaintext)+d.gcmCipher.Overhead())
	n1, err := d.input.Read(ciphertext)
	if err != nil {
		return 0, err
	}

	plaintextBuf, err := d.gcmCipher.Open(nil, d.currentPageIV, ciphertext[:n1], nil)
	if err != nil {
		return 0, err
	}
	n2 := len(plaintextBuf)
	copy(plaintext, plaintextBuf)
	return n2, nil
}

func (d *decrypter) Close() error {
	return d.input.Close()
}

func generatePageIV(pageIndex uint32, pageIVMac hash.Hash, size int) []byte {
	pageIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(pageIndexBytes, pageIndex)
	pageIVMac.Reset()
	pageIVMac.Write(pageIndexBytes)
	iv := pageIVMac.Sum(nil)
	return iv[:size]
}
