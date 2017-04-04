package enc

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
	pageIVMAC hash.Hash
}

// NewEncrypter creates a new Encrypter using the encryption keys.
func NewEncrypter(keys *Keys) (Encrypter, error) {
	gcmCipher, err := newGCMCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	return &encrypter{
		gcmCipher:   gcmCipher,
		pageIVMAC: hmac.New(sha256.New, keys.PageIVSeed),
	}, nil
}

func newGCMCipher(aesKey []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (e *encrypter) Encrypt(plaintext []byte, pageIndex uint32) ([]byte, error) {
	pageIV := generatePageIV(pageIndex, e.pageIVMAC, e.gcmCipher.NonceSize())
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
	pageIVMAC hash.Hash
}

// NewDecrypter creates a new Decrypter instance using the encryption keys.
func NewDecrypter(keys *Keys) (Decrypter, error) {
	gcmCipher, err := newGCMCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	return &decrypter{
		gcmCipher:   gcmCipher,
		pageIVMAC: hmac.New(sha256.New, keys.PageIVSeed),
	}, nil
}

func (d *decrypter) Decrypt(ciphertext []byte, pageIndex uint32) ([]byte, error) {
	pageIV := generatePageIV(pageIndex, d.pageIVMAC, d.gcmCipher.NonceSize())
	return d.gcmCipher.Open(nil, pageIV, ciphertext, nil)
}

func generatePageIV(pageIndex uint32, pageIVMac hash.Hash, size int) []byte {
	pageIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(pageIndexBytes, pageIndex)
	pageIVMac.Reset()
	iv := pageIVMac.Sum(pageIndexBytes)
	return iv[:size]
}
