package api

import (
	"math/rand"
	"testing"

	"fmt"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestValidateMetadata_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	m := &EntryMetadata{
		MediaType:        "application/x-pdf",
		CiphertextSize:   1,
		CiphertextMac:    RandBytes(rng, 32),
		UncompressedSize: 2,
		UncompressedMac:  RandBytes(rng, 32),
	}
	err := ValidateEntryMetadata(m)
	assert.Nil(t, err)
}

func TestValidateMetadata_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	ms := []*EntryMetadata{
		{ // 1
			MediaType:        "",
			CiphertextSize:   1,
			CiphertextMac:    RandBytes(rng, 32),
			UncompressedSize: 2,
			UncompressedMac:  RandBytes(rng, 32),
		},
		{ // 2
			MediaType:        "application/x-pdf",
			CiphertextSize:   0,
			CiphertextMac:    RandBytes(rng, 32),
			UncompressedSize: 2,
			UncompressedMac:  RandBytes(rng, 32),
		},
		{ // 3
			MediaType:        "application/x-pdf",
			CiphertextSize:   1,
			CiphertextMac:    nil,
			UncompressedSize: 2,
			UncompressedMac:  RandBytes(rng, 32),
		},
		{ // 4
			MediaType:        "application/x-pdf",
			CiphertextSize:   1,
			CiphertextMac:    RandBytes(rng, 32),
			UncompressedSize: 0,
			UncompressedMac:  RandBytes(rng, 32),
		},
		{ // 5
			MediaType:        "application/x-pdf",
			CiphertextSize:   1,
			CiphertextMac:    RandBytes(rng, 32),
			UncompressedSize: 2,
			UncompressedMac:  nil,
		},
	}
	for i, m := range ms {
		err := ValidateEntryMetadata(m)
		assert.NotNil(t, err, fmt.Sprintf("case %d", i))
	}
}

func TestMetadata_MarshalLogObject(t *testing.T) {
	oe := zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig())
	rng := rand.New(rand.NewSource(0))
	m := &EntryMetadata{
		MediaType:        "application/x-pdf",
		CiphertextSize:   1,
		CiphertextMac:    RandBytes(rng, 32),
		UncompressedSize: 2,
		UncompressedMac:  RandBytes(rng, 32),
	}
	err := m.MarshalLogObject(oe)
	assert.Nil(t, err)
}
