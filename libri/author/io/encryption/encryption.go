package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

// Encrypter encrypts (compressed) plaintext of a page.
type Encrypter interface {
	// Encrypt encrypts the given plaintext for a given pageIndex, returning the ciphertext.
	Encrypt(plaintext []byte, pageIndex uint32) ([]byte, error)
}

type encrypter struct {
	gcmCipher   cipher.AEAD
	pageIVMACer hash.Hash
}

// NewEncrypter creates a new Encrypter using the encryption keys.
func NewEncrypter(keys *Keys) (Encrypter, error) {
	block, err := aes.NewCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	gcmCipher, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &encrypter{
		gcmCipher:        gcmCipher,
		pageIVMACer:        hmac.New(sha256.New, keys.PageIVSeed),
	}, nil
}

func (e *encrypter) Encrypt(plaintext []byte, pageIndex uint32) ([]byte, error) {
	pageIV := generatePageIV(pageIndex, e.pageIVMACer, e.gcmCipher.NonceSize())
	ciphertext := e.gcmCipher.Seal(nil, pageIV, plaintext, nil)
	return ciphertext, nil
}

// Decrypter decrypts a page's ciphertext.
type Decrypter interface {
	// Decrypt decrypts the ciphertext of a particular page.
	Decrypt(ciphertext []byte, pageIndex uint32) ([]byte, error)
}

type decrypter struct {
	gcmCipher   cipher.AEAD
	pageIVMACer hash.Hash
}

// NewDecrypter creates a new Decrypter instance using the encryption keys.
func NewDecrypter(keys *Keys) (Decrypter, error) {
	block, err := aes.NewCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	gcmCipher, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &decrypter{
		gcmCipher:        gcmCipher,
		pageIVMACer:        hmac.New(sha256.New, keys.PageIVSeed),
	}, nil
}

func (d *decrypter) Decrypt(ciphertext []byte, pageIndex uint32) ([]byte, error) {
	pageIV := generatePageIV(pageIndex, d.pageIVMACer, d.gcmCipher.NonceSize())
	return d.gcmCipher.Open(nil, pageIV, ciphertext, nil)
}

func generatePageIV(pageIndex uint32, pageIVMac hash.Hash, size int) []byte {
	pageIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(pageIndexBytes, pageIndex)
	pageIVMac.Reset()
	iv := pageIVMac.Sum(pageIndexBytes)
	return iv[:size]
}
