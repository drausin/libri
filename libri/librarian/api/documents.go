package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/pkg/errors"
	"github.com/golang/protobuf/proto"
)

const (
	// ECPubKeyLength is the length of a 256-bit ECDSA public key point serialized
	// (uncompressed) to a byte string.
	ECPubKeyLength       = 65

	// DocumentKeyLength is the byte length a document's key.
	DocumentKeyLength = cid.Length

	// AES256KeyLength is the byte length of an AES-256 encryption key.
	AES256KeyLength      = 32

	// PageIVSeedLength is the byte length of the Page block cipher initialization vector (IV)
	// seed.
	PageIVSeedLength     = 32

	// PageHMAC256KeyLength is the byte length of the Page HMAC-256 key.
	PageHMAC256KeyLength = 32

	// MetadataIVLength is the byte length of the metadata block cipher initialization vector.
	MetadataIVLength     = 12

	// EncryptionKeysLength is the total byte length of all the keys used to encrypt an Entry.
	EncryptionKeysLength = AES256KeyLength +
		PageIVSeedLength +
		PageHMAC256KeyLength +
		MetadataIVLength

	// HMAC256Length is the byte length of an HMAC-256.
	HMAC256Length = sha256.Size
)

// GetKey calculates the key from the has of the proto.Message.
func GetKey(value proto.Message) (cid.ID, error) {
	valueBytes, err := proto.Marshal(value)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(valueBytes)
	return cid.FromBytes(hash[:]), nil
}

// ValidateDocument checks that all fields of a Document are populated and have the expected
// lengths.
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

// ValidateEnvelope checks that all fields of an Envelope are populated and have the expected
// lengths.
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
	if err := validateArray(e.EntryKey, DocumentKeyLength, "EntryPublicKey"); err != nil {
		return err
	}
	if err := validateArray(e.EncryptionKeysCiphertext, EncryptionKeysLength,
		"EncryptionKeysLength"); err != nil {
		return err
	}

	return nil
}

// ValidateEntry checks that all fields of an Entry are populated and have the expected byte
// lengths.
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
	if err := validateEntryContents(e); err != nil {
		return err
	}
	return nil
}

func validateEntryContents(e *Entry) error {
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

// ValidatePage checks that all fields of a Page are populated and have the expected lengths.
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

// ValidatePageKeys checks that all fields of a PageKeys are populated and have the expected
// lengths.
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
