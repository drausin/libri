package author

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	logClientIDShort  = "client_id_short"
	logEntryKey       = "entry_key"
	logEnvelopeKey    = "envelope_key"
	logAuthorPubShort = "author_pub_short"
	logReaderPubShort = "reader_pub_short"
	logNPages         = "n_pages"
	logMetadata       = "metadata"
	logSpeedMbps      = "speed_Mbps"
)

func packingContentFields(authorPub []byte) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logAuthorPubShort, id.ShortHex(authorPub[1:9])),
	}
}

func shippingEntryFields(authorPub, readerPub []byte) []zapcore.Field {
	return []zapcore.Field{
		zap.String(logAuthorPubShort, id.ShortHex(authorPub[1:9])),
		zap.String(logReaderPubShort, id.ShortHex(readerPub[1:9])),
	}
}

func uploadedDocFields(
	envKey fmt.Stringer, env *api.Document, md *api.EntryMetadata, elapsed time.Duration,
) []zapcore.Field {
	entryKey := id.FromBytes(env.Contents.(*api.Document_Envelope).Envelope.EntryKey)
	return docFields(envKey, entryKey, md, elapsed)
}

func downloadingDocFields(envKey fmt.Stringer) []zapcore.Field {
	return []zapcore.Field{
		zap.Stringer(logEnvelopeKey, envKey),
	}
}

func downloadedDocFields(
	envKey, entryKey fmt.Stringer, md *api.EntryMetadata, elapsed time.Duration,
) []zapcore.Field {
	return docFields(envKey, entryKey, md, elapsed)
}

func docFields(
	envKey, entryKey fmt.Stringer, md *api.EntryMetadata, elapsed time.Duration,
) []zapcore.Field {

	// surprisingly hard to truncate this to 2 decimal places as float
	speedMbps := float32(md.UncompressedSize) * 8 / float32(2<<20) / float32(elapsed.Seconds())
	return []zapcore.Field{
		zap.Stringer(logEnvelopeKey, envKey),
		zap.Stringer(logEntryKey, entryKey),
		zap.String(logSpeedMbps, fmt.Sprintf("%.2f", speedMbps)),
		zap.Object(logMetadata, md),
	}
}

func unpackingContentFields(entryKey fmt.Stringer, nPages int) []zapcore.Field {
	return []zapcore.Field{
		zap.Stringer(logEntryKey, entryKey),
		zap.Int(logNPages, nPages),
	}
}

func sharingDocFields(envKey fmt.Stringer, readerPub *ecdsa.PublicKey) []zapcore.Field {
	return []zapcore.Field{
		zap.Stringer(logEnvelopeKey, envKey),
		zap.String(logReaderPubShort, id.ShortHex(ecid.ToPublicKeyBytes(readerPub)[1:9])),
	}
}

func sharedDocFields(envKey, entryKey fmt.Stringer, authorPub, readerPub []byte) []zapcore.Field {
	return []zapcore.Field{
		zap.Stringer(logEntryKey, entryKey),
		zap.Stringer(logEnvelopeKey, envKey),
		zap.String(logAuthorPubShort, id.ShortHex(authorPub[1:9])),
		zap.String(logReaderPubShort, id.ShortHex(readerPub[1:9])),
	}
}
