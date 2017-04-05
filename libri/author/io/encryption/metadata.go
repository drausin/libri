package encryption

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"bytes"
	"errors"
)

// ErrUnexpectedMAC occurs when the calculated MAC did not match the expected MAC.
var ErrUnexpectedMAC = errors.New("unexpected MAC")

// EncryptedMetadata contains both the ciphertext and ciphertext MAC involved in encrypting an
// *api.Metadata instance.
type EncryptedMetadata struct {
	Ciphertext []byte
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
		Ciphertext: ciphertext,
		CiphertextMAC: ciphertextMAC,
	}, nil
}

// EncryptMetadata encrypts an *api.Metadata instance using the AES key and the MetadataIV key for
// the MAC.
func EncryptMetadata(m *api.Metadata, keys *Keys) (*EncryptedMetadata, error) {
	mPlaintext, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	cipher, err := newGCMCipher(keys.AESKey)
	if err != nil {
		return nil, err
	}
	mCiphertext := cipher.Seal(nil, keys.MetadataIV, mPlaintext, nil)
	mac, err := HMAC(mCiphertext, keys.HMACKey)
	if err != nil {
		return nil, err
	}
	return NewEncryptedMetadata(mCiphertext, mac)
}

// DecryptMetadata decryptes an *EncryptedMetadata instance, using the AES key and the MetadataIV
// key. It returns UnexpectedMACErr if the calculated ciphertext MAC does not match the expected
// ciphertext MAC.
func DecryptMetadata(em *EncryptedMetadata, keys *Keys) (*api.Metadata, error) {
	mac, err := HMAC(em.Ciphertext, keys.HMACKey)
	if err != nil {
		return nil, err
	}
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
	m := &api.Metadata{}
	if err := proto.Unmarshal(mPlaintext, m); err != nil {
		return nil, err
	}
	return m, nil
}
