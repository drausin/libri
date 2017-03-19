package encryption

import (
	"bytes"
	crand "crypto/rand"
	"github.com/drausin/libri/libri/librarian/api"
	"io"
	mrand "math/rand"
)

// Keys are used to encrypt an Entry and its Pages.
type Keys struct {
	// AESKey is the 32-byte AES-256 key used to encrypt Pages and Entry metadata.
	AESKey      []byte

	// PageIVSeed is the 32-byte block cipher initialization vector (IV) seed for Page
	// encryption.
	PageIVSeed  []byte

	// PageHMACKey is the 32-byte key used for Page HMAC-256 calculations.
	PageHMACKey []byte

	// MetadataIV is the 12-byte IV for the Entry metadata block cipher.
	MetadataIV  []byte
}

// NewRandom creates new Keys using crypto.Rand as a source of entropy.
func NewRandom() *Keys {
	return newRandom(crand.Reader)
}

// NewPseudoRandom creates new Keys using math.Rand as a source of entropy.
func NewPseudoRandom(rng *mrand.Rand) *Keys {
	return newRandom(rng)
}

func newRandom(r io.Reader) *Keys {
	buf := make([]byte, api.EncryptionKeysLength)
	_, err := r.Read(buf)
	if err != nil {
		panic(err)
	}
	keys, err := Unmarshal(buf)
	if err != nil {
		panic(err)
	}

	return keys
}

// Marshal serializes the Keys to their byte representation.
func Marshal(keys *Keys) []byte {
	return bytes.Join(
		[][]byte{
			keys.AESKey,
			keys.PageIVSeed,
			keys.PageHMACKey,
			keys.MetadataIV,
		},
		[]byte{},
	)
}

// Unmarshal deserializes the Keys from their byte representation.
func Unmarshal(x []byte) (*Keys, error) {
	err := api.ValidateBytes(x, api.EncryptionKeysLength, "Keys byte representation")
	if err != nil {
		return nil, err
	}
	var offset int
	return &Keys{
		AESKey:      next(x, &offset, api.AESKeyLength),
		PageIVSeed:  next(x, &offset, api.PageIVSeedLength),
		PageHMACKey: next(x, &offset, api.PageHMACKeyLength),
		MetadataIV:  next(x, &offset, api.MetadataIVLength),
	}, nil
}

func next(x []byte, offset *int, len int) []byte {
	next := x[*offset : *offset+len]
	*offset += len
	return next
}
