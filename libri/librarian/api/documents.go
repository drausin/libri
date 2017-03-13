package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/drausin/libri/libri/common/id"
	"github.com/pkg/errors"
)

const (
	ECPubKeyLength       = 65
	EntryKeyLength       = id.Length
	AES256KeyLength      = 32
	PageIVSeedLength     = 32
	PageHMAC256KeyLength = 32
	MetadataIVLength     = 12
	EncryptionKeysLength = AES256KeyLength +
		PageIVSeedLength +
		PageHMAC256KeyLength +
		MetadataIVLength
	HMAC256Length = sha256.Size
)

func ValidateDocument(d *Document) error {
	if d == nil {
		return errors.New("Document may not be nil")
	}
	switch x := d.Contents.(type) {
	case *Document_Envelope:
		return ValidateEnvelope(x.Envelope)
	case *Document_Entry:
		return ValidateEntry(x.Entry)
	case *Document_Page:
		return ValidatePage(x.Page)
	default:
		return errors.New("unknown document type")
	}
}

func ValidateEnvelope(e *Envelope) error {
	if e == nil {
		return errors.New("Envelope may not be nil")
	}
	if err := validateArray(e.AuthorPublicKey, ECPubKeyLength, "AuthorPublicKey"); err != nil {
		return err
	}
	if err := validateArray(e.ReaderPublicKey, ECPubKeyLength, "ReaderPublicKey"); err != nil {
		return err
	}
	if err := validateArray(e.EntryKey, EntryKeyLength, "EntryPublicKey"); err != nil {
		return err
	}
	if err := validateArray(e.EncryptionKeysCiphertext, EncryptionKeysLength,
		"EncryptionKeysLength"); err != nil {
		return err
	}

	return nil
}

func ValidateEntry(e *Entry) error {
	if e == nil {
		return errors.New("Entry may not be nil")
	}
	if err := validateArray(e.AuthorPublicKey, ECPubKeyLength, "AuthorPublicKey"); err != nil {
		return err
	}
	if e.CreatedTime == 0 {
		return errors.New("CreateTime must be populated")
	}
	if err := validateArray(e.MetadataCiphertextMac, HMAC256Length,
		"MetadataCiphertextMac"); err != nil {
		return err
	}
	if err := validateNotEmpty(e.MetadataCiphertext, "MetadataCiphertext"); err != nil {
		return err
	}
	if err := validateArray(e.ContentsCiphertextMac, HMAC256Length,
		"ContentsCiphertextMac"); err != nil {
		return err
	}
	if e.ContentsCiphertextSize == 0 {
		return errors.New("ContentsCiphertextSize must be greateer than zero")
	}
	switch x := e.Contents.(type) {
	case *Entry_Page:
		if !bytes.Equal(e.AuthorPublicKey, x.Page.AuthorPublicKey) {
			return errors.New("Page author public key must be the same as Entry's")
		}
		return ValidatePage(x.Page)
	case *Entry_PageKeys:
		return ValidatePageKeys(x.PageKeys)
	default:
		return errors.New("unknown Entry.Contents type")
	}
}

func ValidatePage(p *Page) error {
	if p == nil {
		return errors.New("Page may not be nil")
	}
	if err := validateArray(p.AuthorPublicKey, ECPubKeyLength, "AuthorPublicKey"); err != nil {
		return err
	}
	// nothing to check for index, since it's zero value is legitimate
	if err := validateArray(p.CiphertextMac, HMAC256Length, "CiphertextMac"); err != nil {
		return err
	}
	if err := validateNotEmpty(p.Ciphertext, "Ciphertext"); err != nil {
		return err
	}
	return nil
}

func ValidatePageKeys(pk *PageKeys) error {
	if pk == nil {
		return errors.New("PageKeys may not be nil")
	}
	if pk.Keys == nil {
		return errors.New("PageKeys.Keys may not be nil")
	}
	if len(pk.Keys) == 0 {
		return errors.New("PageKeys.Keys must have length > 0")
	}
	for i, k := range pk.Keys {
		if err := validateNotEmpty(k, fmt.Sprintf("key %d", i)); err != nil {
			return err
		}
	}
	return nil
}

func validateArray(value []byte, expectedLen int, name string) error {
	if err := validateNotEmpty(value, name); err != nil {
		return err
	}
	if len(value) != expectedLen {
		return fmt.Errorf("%s must have length %d, found length %d", name, expectedLen,
			len(value))
	}
	return nil
}

func validateNotEmpty(value []byte, name string) error {
	if value == nil {
		return fmt.Errorf("%s may not be nil", name)
	}
	if len(value) == 0 {
		return fmt.Errorf("%s must have length > 0", name)
	}
	if allZeros(value) {
		return fmt.Errorf("%s may not be all zeros", name)
	}
	return nil
}

func allZeros(value []byte) bool {
	for _, v := range value {
		if v != 0 {
			return false
		}
	}
	return true
}
