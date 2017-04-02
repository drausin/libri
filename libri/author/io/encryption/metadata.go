package encryption

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
	"bytes"
	"github.com/pkg/errors"
)

var UnexpectedMACErr = errors.New("unexpected MAC")

type EncryptedMetadata struct {
	Ciphertext []byte
	CiphertextMAC []byte
}

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
	macer := NewHMACer(keys.HMACKey)
	return NewEncryptedMetadata(mCiphertext, macer.Sum(mCiphertext))
}

func DecryptMetadata(em *EncryptedMetadata, keys *Keys) (*api.Metadata, error) {
	macer := NewHMACer(keys.HMACKey)
	if !bytes.Equal(em.CiphertextMAC, macer.Sum(em.Ciphertext)) {
		return nil, UnexpectedMACErr
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
