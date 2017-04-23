package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"log"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/golang/protobuf/proto"
	"errors"
)

// field lengths
const (
	// ECPubKeyLength is the length of a 256-bit ECDSA public key point serialized
	// (uncompressed) to a byte string.
	ECPubKeyLength = 65

	// DocumentKeyLength is the byte length a document's key.
	DocumentKeyLength = cid.Length

	// AESKeyLength is the byte length of an AES-256 encryption key.
	AESKeyLength = 32

	// PageIVSeedLength is the byte length of the Page block cipher initialization vector (IV)
	// seed.
	PageIVSeedLength = 32

	// HMACKeyLength is the byte length of the Page HMAC-256 key.
	HMACKeyLength = 32

	// MetadataIVLength is the byte length of the metadata block cipher initialization vector.
	MetadataIVLength = 12

	// EncryptionKeysLength is the total byte length of all the keys used to encrypt an Entry.
	EncryptionKeysLength = AESKeyLength +
		PageIVSeedLength +
		HMACKeyLength +
		MetadataIVLength

	// HMAC256Length is the byte length of an HMAC-256.
	HMAC256Length = sha256.Size
)

var (
	// ErrUnexpectedDocumentType indicates when a document type is not expected (e.g., a Page
	// when expecting an Entry).
	ErrUnexpectedDocumentType = errorsNew("unexpected document type")

	// ErrUnknownDocumentType indicates when a document type is not known (usually, this
	// error should never actually be thrown).
	ErrUnknownDocumentType = errorsNew("unknown document type")
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

// GetAuthorPub returns the author public key for a given document.
func GetAuthorPub(d *Document) []byte {
	switch c := d.Contents.(type) {
	case *Document_Entry:
		return c.Entry.AuthorPublicKey
	case *Document_Page:
		return c.Page.AuthorPublicKey
	case *Document_Envelope:
		return c.Envelope.AuthorPublicKey
	}
	panic(ErrUnknownDocumentType)
}

// GetEntryPageKeys returns the []id.ID page keys if the entry is multi-page. It returns nil for
// single-page entries.
func GetEntryPageKeys(entry *Document) ([]cid.ID, error) {
	switch x := entry.Contents.(*Document_Entry).Entry.Contents.(type) {
	case *Entry_PageKeys:
		pageKeys := make([]cid.ID, len(x.PageKeys.Keys))
		for i, keyBytes := range x.PageKeys.Keys {
			pageKeys[i] = cid.FromBytes(keyBytes)
		}
		return pageKeys, nil
	case *Entry_Page:
		return nil, nil
	}
	return nil, ErrUnexpectedDocumentType
}

// GetPageDocument wraps a Page into a Document, returning it and its key.
func GetPageDocument(page *Page) (*Document, cid.ID, error) {
	pageDoc := &Document{
		Contents: &Document_Page{
			Page: page,
		},
	}
	// store single page as separate doc
	pageKey, err := GetKey(pageDoc)
	if err != nil {
		// should never happen
		return nil, nil, err
	}
	return pageDoc, pageKey, nil
}

// ValidateDocument checks that all fields of a Document are populated and have the expected
// lengths.
func ValidateDocument(d *Document) error {
	if d == nil {
		return errorsNew("Document may not be nil")
	}
	switch c := d.Contents.(type) {
	case *Document_Envelope:
		return ValidateEnvelope(c.Envelope)
	case *Document_Entry:
		return ValidateEntry(c.Entry)
	case *Document_Page:
		return ValidatePage(c.Page)
	}
	return ErrUnknownDocumentType
}

// ValidateEnvelope checks that all fields of an Envelope are populated and have the expected
// lengths.
func ValidateEnvelope(e *Envelope) error {
	if e == nil {
		return errorsNew("Envelope may not be nil")
	}
	if err := ValidatePublicKey(e.AuthorPublicKey); err != nil {
		return err
	}
	if err := ValidatePublicKey(e.ReaderPublicKey); err != nil {
		return err
	}
	if err := ValidateBytes(e.EntryKey, DocumentKeyLength, "EntryKey"); err != nil {
		return err
	}
	return nil
}

// ValidateEntry checks that all fields of an Entry are populated and have the expected byte
// lengths.
func ValidateEntry(e *Entry) error {
	if e == nil {
		return errorsNew("Entry may not be nil")
	}
	if err := ValidatePublicKey(e.AuthorPublicKey); err != nil {
		return err
	}
	if e.CreatedTime == 0 {
		return errorsNew("CreateTime must be populated")
	}
	if err := ValidateHMAC256(e.MetadataCiphertextMac); err != nil {
		log.Print("metadata ciphertext mac issue")
		return err
	}
	if err := ValidateNotEmpty(e.MetadataCiphertext, "MetadataCiphertext"); err != nil {
		return err
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
			return errorsNew("Page author public key must be the same as Entry's")
		}
		return ValidatePage(x.Page)
	case *Entry_PageKeys:
		return ValidatePageKeys(x.PageKeys)
	default:
		return errorsNew("unknown Entry.Contents type")
	}
}

// ValidatePage checks that all fields of a Page are populated and have the expected lengths.
func ValidatePage(p *Page) error {
	if p == nil {
		return errorsNew("Page may not be nil")
	}
	if err := ValidatePublicKey(p.AuthorPublicKey); err != nil {
		return err
	}
	// nothing to check for index, since it's zero value is legitimate
	if err := ValidateHMAC256(p.CiphertextMac); err != nil {
		log.Print("page ciphertext mac issue")
		return err
	}
	if err := ValidateNotEmpty(p.Ciphertext, "Ciphertext"); err != nil {
		return err
	}
	return nil
}

// ValidatePageKeys checks that all fields of a PageKeys are populated and have the expected
// lengths.
func ValidatePageKeys(pk *PageKeys) error {
	if pk == nil {
		return errorsNew("PageKeys may not be nil")
	}
	if pk.Keys == nil {
		return errorsNew("PageKeys.Keys may not be nil")
	}
	if len(pk.Keys) == 0 {
		return errorsNew("PageKeys.Keys must have length > 0")
	}
	for i, k := range pk.Keys {
		if err := ValidateNotEmpty(k, fmt.Sprintf("key %d", i)); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePublicKey checks that a value can be a 256-bit elliptic curve public key.
func ValidatePublicKey(value []byte) error {
	return ValidateBytes(value, ECPubKeyLength, "PublicKey")
}

// ValidateAESKey checks the a value can be a 256-bit AES key.
func ValidateAESKey(value []byte) error {
	return ValidateBytes(value, AESKeyLength, "AESKey")
}

// ValidateHMACKey checks that a value can be an HMAC-256 key.
func ValidateHMACKey(value []byte) error {
	return ValidateBytes(value, HMACKeyLength, "HMACKey")
}

// ValidatePageIVSeed checks that a value can be a 256-bit initialization vector seed.
func ValidatePageIVSeed(value []byte) error {
	return ValidateBytes(value, PageIVSeedLength, "PageIVSeed")
}

// ValidateMetadataIV checks that a value can be a 12-byte GCM initialization vector.
func ValidateMetadataIV(value []byte) error {
	return ValidateBytes(value, MetadataIVLength, "MetadataIV")
}

// ValidateHMAC256 checks that a value can be an HMAC-256.
func ValidateHMAC256(value []byte) error {
	return ValidateBytes(value, HMAC256Length, "HMAC256")
}

// ValidateBytes returns whether the byte slice is not empty and has an expected length.
func ValidateBytes(value []byte, expectedLen int, name string) error {
	if err := ValidateNotEmpty(value, name); err != nil {
		return err
	}
	if len(value) != expectedLen {
		return fmt.Errorf("%s must have length %d, found length %d", name, expectedLen,
			len(value))
	}
	return nil
}

// ValidateNotEmpty returns whether the byte slice is not empty.
func ValidateNotEmpty(value []byte, name string) error {
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
