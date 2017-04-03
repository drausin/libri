package api

import (
	"encoding/binary"
	"github.com/pkg/errors"
)

// required Entry metadata fields
const (
	metadataEntryPrefix = "libri.entry."

	// MetadataEntryMediaType indicates the media type.
	MetadataEntryMediaType = metadataEntryPrefix + "media_type"

	// MetadataEntryCiphertextSize indicates the total ciphertext size across all pages.
	MetadataEntryCiphertextSize = metadataEntryPrefix + "ciphertext_size"

	// MetadataEntryCiphertextMAC indicates the MAC of the entire ciphertext.
	MetadataEntryCiphertextMAC = metadataEntryPrefix + "ciphertext_mac"

	// MetadataEntryUncompressedSize indicates the total size of the entire uncompressed entry.
	MetadataEntryUncompressedSize = metadataEntryPrefix + "uncompressed_size"

	// MetadataEntryUncompressedMAC indicates the MAC of the entire uncompressed entry.
	MetadataEntryUncompressedMAC = metadataEntryPrefix + "uncompressed_mac"
)

// optional Entry metadata fields
const (
	// MetadataEntryFilepath indicates the (relative) filepath of the data contained in the
	// entry.
	MetadataEntryFilepath = metadataEntryPrefix + "filepath"

	// MetadataEntrySchema indicates the schema (however defined) of the data contained in the
	// entry.
	MetadataEntrySchema = metadataEntryPrefix + "schema"
)

var (
	// UnexpectedZeroErr describes when an error is unexpectedly zero.
	UnexpectedZeroErr = errors.New("unexpected zero value")
)

// NewEntryMetadata creates a new *Metadata instance with the given (required) fields.
func NewEntryMetadata(
	mediaType string,
	ciphertextSize uint64,
	ciphertextMAC []byte,
	uncompressedSize uint64,
	uncompressedMAC []byte,
) (*Metadata, error) {
	if mediaType == "" {
		return nil, UnexpectedZeroErr
	}
	if ciphertextSize == 0 {
		return nil, UnexpectedZeroErr
	}
	if err := ValidateHMAC256(ciphertextMAC); err != nil {
		return nil, err
	}
	if uncompressedSize == 0 {
		return nil, UnexpectedZeroErr
	}
	if err := ValidateHMAC256(uncompressedMAC); err != nil {
		return nil, err
	}
	return &Metadata{
		Properties: map[string][]byte{
			MetadataEntryMediaType:        []byte(mediaType),
			MetadataEntryCiphertextSize:   uint64Bytes(ciphertextSize),
			MetadataEntryCiphertextMAC:    ciphertextMAC,
			MetadataEntryUncompressedSize: uint64Bytes(uncompressedSize),
			MetadataEntryUncompressedMAC:  uncompressedMAC,
		},
	}, nil
}

func (m Metadata) GetMediaType() (string, bool) {
	return m.GetString(MetadataEntryMediaType)
}

func (m Metadata) GetCiphertextSize() (uint64, bool) {
	return m.GetUint64(MetadataEntryCiphertextSize)
}

func (m Metadata) GetCiphertextMAC() ([]byte, bool) {
	return m.GetBytes(MetadataEntryCiphertextMAC)
}

func (m Metadata) GetUncompressedSize() (uint64, bool) {
	return m.GetUint64(MetadataEntryUncompressedSize)
}

func (m Metadata) GetUncompressedMAC() ([]byte, bool) {
	return m.GetBytes(MetadataEntryUncompressedMAC)
}

func (m Metadata) GetBytes(key string) ([]byte, bool) {
	value, in := m.Properties[key]
	return value, in
}

func (m Metadata) SetBytes(key string, value []byte) {
	m.Properties[key] = value
}

func (m Metadata) GetString(key string) (string, bool) {
	value, in := m.Properties[key]
	return string(value), in
}

func (m Metadata) SetString(key string, value string) {
	m.Properties[key] = []byte(value)
}

func (m Metadata) GetUint64(key string) (uint64, bool) {
	value, in := m.Properties[key]
	if !in {
		return 0, false
	}
	return binary.BigEndian.Uint64(value), true
}

func (m Metadata) SetUint64(key string, value uint64) {
	m.Properties[key] = uint64Bytes(value)
}

func uint64Bytes(value uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value)
	return b
}
