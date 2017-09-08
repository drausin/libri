package api

import (
	"errors"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap/zapcore"
)

const (
	// logging keys
	logMediaType             = "media_type"
	logCompressionCodec      = "compression_codec"
	logCiphertextSize        = "ciphertext_size"
	logCiphertextSizeHuman   = "ciphertext_size_human"
	logUncompressedSize      = "uncompressed_size"
	logUncompressedSizeHuman = "uncompressed_size_human"
)

var (
	// ErrUnexpectedZero describes when an error is unexpectedly zero.
	ErrUnexpectedZero = errors.New("unexpected zero value")
)

// ValidateEntryMetadata checks that the metadata has all the required non-zero values.
func ValidateEntryMetadata(m *EntryMetadata) error {
	if m.MediaType == "" {
		return ErrUnexpectedZero
	}
	if m.CiphertextSize == 0 {
		return ErrUnexpectedZero
	}
	if err := ValidateHMAC256(m.CiphertextMac); err != nil {
		return err
	}
	if m.UncompressedSize == 0 {
		return ErrUnexpectedZero
	}
	if err := ValidateHMAC256(m.UncompressedMac); err != nil {
		return err
	}
	return nil
}

// MarshalLogObject converts the metadata into an object (which will become json) for logging.
func (m *EntryMetadata) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString(logMediaType, m.MediaType)
	oe.AddString(logCompressionCodec, m.CompressionCodec.String())
	oe.AddUint64(logCiphertextSize, m.CiphertextSize)
	oe.AddString(logCiphertextSizeHuman, humanize.Bytes(m.CiphertextSize))
	oe.AddUint64(logUncompressedSize, m.UncompressedSize)
	oe.AddString(logUncompressedSizeHuman, humanize.Bytes(m.UncompressedSize))
	return nil
}
