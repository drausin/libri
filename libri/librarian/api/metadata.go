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
	// ErrUnexpectedZero describes when an error is unexpectedly zero.
	ErrUnexpectedZero = errors.New("unexpected zero value")
)

// NewEntryMetadata creates a new *Metadata instance with the given (required) fields.
func NewEntryMetadata(
	mediaType string, // TODO (drausin) change to compression codec
	ciphertextSize uint64,
	ciphertextMAC []byte,
	uncompressedSize uint64,
	uncompressedMAC []byte,
) (*Metadata, error) {
	m := &Metadata{
		Properties: map[string][]byte{
			MetadataEntryMediaType:        []byte(mediaType),
			MetadataEntryCiphertextSize:   uint64Bytes(ciphertextSize),
			MetadataEntryCiphertextMAC:    ciphertextMAC,
			MetadataEntryUncompressedSize: uint64Bytes(uncompressedSize),
			MetadataEntryUncompressedMAC:  uncompressedMAC,
		},
	}
	if err := ValidateMetadata(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ValidateMetadata checks that the metadata has all the required non-zero values.
func ValidateMetadata(m *Metadata) error {
	if value, _ := m.GetMediaType(); value == "" {
		return ErrUnexpectedZero
	}
	if value, _ := m.GetCiphertextSize(); value == 0 {
		return ErrUnexpectedZero
	}
	if value, _ := m.GetCiphertextMAC(); ValidateHMAC256(value) != nil {
		return ValidateHMAC256(value)
	}
	if value, _ := m.GetUncompressedSize(); value == 0 {
		return ErrUnexpectedZero
	}
	if value, _ := m.GetUncompressedMAC(); ValidateHMAC256(value) != nil {
		return ValidateHMAC256(value)
	}
	return nil
}

// GetMediaType returns the media type.
func (m *Metadata) GetMediaType() (string, bool) {
	return m.GetString(MetadataEntryMediaType)
}

// GetCiphertextSize returns the size of the ciphertext.
func (m *Metadata) GetCiphertextSize() (uint64, bool) {
	return m.GetUint64(MetadataEntryCiphertextSize)
}

// GetCiphertextMAC returns the ciphertext MAC.
func (m *Metadata) GetCiphertextMAC() ([]byte, bool) {
	return m.GetBytes(MetadataEntryCiphertextMAC)
}

// GetUncompressedSize returns the size of the uncompressed data.
func (m *Metadata) GetUncompressedSize() (uint64, bool) {
	return m.GetUint64(MetadataEntryUncompressedSize)
}

// GetUncompressedMAC returns the MAC of the uncompressed data.
func (m *Metadata) GetUncompressedMAC() ([]byte, bool) {
	return m.GetBytes(MetadataEntryUncompressedMAC)
}

// GetBytes returns the byte slice value for a given key.
func (m *Metadata) GetBytes(key string) ([]byte, bool) {
	value, in := m.Properties[key]
	return value, in
}

// SetBytes sets the byte slice value for a given key.
func (m *Metadata) SetBytes(key string, value []byte) {
	m.Properties[key] = value
}

// GetString returns the string value for a given key.
func (m *Metadata) GetString(key string) (string, bool) {
	value, in := m.Properties[key]
	return string(value), in
}

// SetString sets the string value for a given key.
func (m *Metadata) SetString(key string, value string) {
	m.Properties[key] = []byte(value)
}

// GetUint64 returns the uint64 value for a given key.
func (m *Metadata) GetUint64(key string) (uint64, bool) {
	value, in := m.Properties[key]
	if !in {
		return 0, false
	}
	return binary.BigEndian.Uint64(value), true
}

// SetUint64 sets the uint64 value for a given key.
func (m *Metadata) SetUint64(key string, value uint64) {
	m.Properties[key] = uint64Bytes(value)
}

func uint64Bytes(value uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value)
	return b
}
