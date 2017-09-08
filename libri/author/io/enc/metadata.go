package enc

import (
	"bytes"
	"errors"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
)

// ErrUnexpectedMAC occurs when the calculated MAC did not match the expected MAC.
var ErrUnexpectedMAC = errors.New("unexpected MAC")

// EncryptedMetadata contains both the ciphertext and ciphertext MAC involved in encrypting an
// *api.EntryMetadata instance.
type EncryptedMetadata struct {
	Ciphertext    []byte
	CiphertextMAC []byte
}

// NewEncryptedMetadata creates a new *EncryptedMetadata instance if the ciphertext and
// ciphertextMAC are valid.
func NewEncryptedMetadata(ciphertext, ciphertextMAC []byte) (*EncryptedMetadata, error) {
	if err := api.ValidateNotEmpty(ciphertext, "MetadataCiphertext"); err != nil {
		return nil, err
	}
	if err := api.ValidateHMAC256(ciphertextMAC); err != nil {
		return nil, err
	}
	return &EncryptedMetadata{
		Ciphertext:    ciphertext,
		CiphertextMAC: ciphertextMAC,
	}, nil
}

// EntryMetadataEncrypter encrypts *api.EntryMetadata.
type EntryMetadataEncrypter interface {
	// EncryptMetadata encrypts an *api.EntryMetadata instance using the AES key and the MetadataIV
	// key for the MAC.
	Encrypt(m *api.EntryMetadata, keys *EEK) (*EncryptedMetadata, error)
}

// MetadataDecrypter decrypts *EncryptedMetadata.
type MetadataDecrypter interface {
	// Decrypt decrypts an *EncryptedMetadata instance, using the AES key and the MetadataIV
	// key. It returns UnexpectedMACErr if the calculated ciphertext MAC does not match the
	// expected ciphertext MAC.
	Decrypt(em *EncryptedMetadata, keys *EEK) (*api.EntryMetadata, error)
}

// MetadataEncrypterDecrypter encrypts *api.EntryMetadata and decrypts *EncryptedMetadata.
type MetadataEncrypterDecrypter interface {
	EntryMetadataEncrypter
	MetadataDecrypter
}

type metadataEncDec struct{}

// NewMetadataEncrypterDecrypter creates a new MetadataEncrypterDecrypter.
func NewMetadataEncrypterDecrypter() MetadataEncrypterDecrypter {
	return metadataEncDec{}
}

func (metadataEncDec) Encrypt(m *api.EntryMetadata, keys *EEK) (*EncryptedMetadata, error) {
	mPlaintext, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	cipher, err := newGCMCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	mCiphertext := cipher.Seal(nil, keys.MetadataIV, mPlaintext, nil)
	return NewEncryptedMetadata(mCiphertext, HMAC(mCiphertext, keys.HMACKey))
}

func (metadataEncDec) Decrypt(em *EncryptedMetadata, keys *EEK) (*api.EntryMetadata, error) {
	mac := HMAC(em.Ciphertext, keys.HMACKey)
	if !bytes.Equal(em.CiphertextMAC, mac) {
		return nil, ErrUnexpectedMAC
	}
	cipher, err := newGCMCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	mPlaintext, err := cipher.Open(nil, keys.MetadataIV, em.Ciphertext, nil)
	if err != nil {
		return nil, err
	}
	m := &api.EntryMetadata{}
	if err := proto.Unmarshal(mPlaintext, m); err != nil {
		return nil, err
	}
	return m, nil
}
