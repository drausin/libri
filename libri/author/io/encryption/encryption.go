package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

type Encrypter interface {
	Encrypt(plaintext []byte, page uint32) ([]byte, error)
}

type encrypter struct {
	gcmCipher   cipher.AEAD
	pageIVMACer hash.Hash
}

func NewEncrypter(keys *Keys) (Encrypter, error) {
	// TODO (drausin) pass in just keys needed + validate
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

func (e *encrypter) Encrypt(plaintext []byte, page uint32) ([]byte, error) {
	pageIV := generatePageIV(page, e.pageIVMACer, e.gcmCipher.NonceSize())
	ciphertext := e.gcmCipher.Seal(nil, pageIV, plaintext, nil)
	return ciphertext, nil
}


type Decrypter interface {
	Decrypt(ciphertext []byte, page uint32) ([]byte, error)
}

type decrypter struct {
	gcmCipher   cipher.AEAD
	pageIVMACer hash.Hash
}

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

func (d *decrypter) Decrypt(ciphertext []byte, page uint32) ([]byte, error) {
	pageIV := generatePageIV(page, d.pageIVMACer, d.gcmCipher.NonceSize())
	return d.gcmCipher.Open(nil, pageIV, ciphertext, nil)
}

func generatePageIV(pageIndex uint32, pageIVMac hash.Hash, size int) []byte {
	pageIndexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(pageIndexBytes, pageIndex)
	pageIVMac.Reset()
	iv := pageIVMac.Sum(pageIndexBytes)
	return iv[:size]
}
