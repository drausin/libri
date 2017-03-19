package encryption

import (
	"bytes"
	crand "crypto/rand"
	"github.com/drausin/libri/libri/librarian/api"
	"io"
	mrand "math/rand"
)

type Keys struct {
	AESKey      []byte
	PageIVSeed  []byte
	PageHMACKey []byte
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
