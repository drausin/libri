package api

import "encoding/binary"

// required Entry metadata fields
const (
	MetadataEntryPrefix = "libri.entry."
	MetadataEntryMediaType           = MetadataEntryPrefix + "media_type"
	MetadataEntryCiphertextSize      = MetadataEntryPrefix + "ciphertext_size"
	MetadataEntryCiphertextMAC       = MetadataEntryPrefix + "ciphertext_mac"
	MetadataEntryUncompressedSize    = MetadataEntryPrefix + "uncompressed_size"
	MetadataEntryUncompressedMAC     = MetadataEntryPrefix + "uncompressed_mac"
)

// optional Entry metadata fields
const (
	EntryFilepath = MetadataEntryPrefix + "filepath"
	EntrySchema   = MetadataEntryPrefix + "schema"
)

func NewEntryMetadata(
	mediaType string,
	ciphertextSize uint64,
	ciphertextMAC []byte,
	uncompressedSize uint64,
	uncompressedMAC []byte,
) *Metadata {
	return &Metadata{
		Properties: map[string][]byte{
			MetadataEntryMediaType: []byte(mediaType),
			MetadataEntryCiphertextSize: uint64Bytes(ciphertextSize),
			MetadataEntryCiphertextMAC: ciphertextMAC,
			MetadataEntryUncompressedSize: uint64Bytes(uncompressedSize),
			MetadataEntryUncompressedMAC: uncompressedMAC,
		},
	}
}

func (m Metadata) EntryMediaType() (string, bool) {
	return m.GetString(MetadataEntryMediaType)
}

func (m Metadata) EntryCiphertextSize() (uint64, bool) {
	return m.GetUint64(MetadataEntryCiphertextSize)
}

func (m Metadata) EntryCiphertextMAC() ([]byte, bool) {
	return m.GetBytes(MetadataEntryCiphertextMAC)
}

func (m Metadata) EntryUncompressedSize() (uint64, bool) {
	return m.GetUint64(MetadataEntryCiphertextSize)
}

func (m Metadata) EntryUncompressedMAC() ([]byte, bool) {
	return m.GetBytes(MetadataEntryCiphertextMAC)
}

func (m Metadata) GetBytes(key string) ([]byte, bool) {
	value, in := m.Properties[key]
	return value, in
}

func (m Metadata) GetString(key string) (string, bool) {
	value, in := m.Properties[key]
	return string(value), in
}

func (m Metadata) GetUint64(key string) (uint64, bool) {
	value, in := m.Properties[key]
	if !in {
		return 0, false
	}
	return binary.BigEndian.Uint64(value), true
}

func uint64Bytes(value uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value)
	return b
}

