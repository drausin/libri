package enc

import (
	"bytes"
	"crypto/ecdsa"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/hkdf"
	"crypto/sha256"
	"math/rand"
)

var AuthorOffCurveErr = errors.New("author public key point not on expected elliptic curve")
var ReaderOffCurveErr = errors.New("reader public key point not on expected elliptic curve")
var IncompleteKeyDefinitionErr = errors.New("incomplete key definition")

// Keys are used to encrypt an Entry and its Pages.
type Keys struct {
	// AESKey is the 32-byte AES-256 key used to encrypt Pages and Entry metadata.
	AESKey []byte

	// PageIVSeed is the 32-byte block cipher initialization vector (IV) seed for Page
	// enc.
	PageIVSeed []byte

	// HMACKey is the 32-byte key used for Page HMAC-256 calculations.
	HMACKey []byte

	// MetadataIV is the 12-byte IV for the Entry metadata block cipher.
	MetadataIV []byte
}

func NewKeys(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey) (*Keys, error) {
	if !ecid.Curve.IsOnCurve(priv.X, priv.Y) {
		return nil, AuthorOffCurveErr
	}
	if !ecid.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, ReaderOffCurveErr
	}
	secretX, _ := ecid.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	kdf := hkdf.New(sha256.New, secretX.Bytes(), nil, nil)
	keyBytes := make([]byte, api.EncryptionKeysLength)
	n, err := kdf.Read(keyBytes)
	if err != nil {
		return nil, err
	}
	if n != api.EncryptionKeysLength {
		return nil, IncompleteKeyDefinitionErr
	}
	return Unmarshal(keyBytes)
}

func NewPseudoRandomKeys(rng *rand.Rand) *Keys {
	authorPriv := ecid.NewPseudoRandom(rng)
	readerPriv := ecid.NewPseudoRandom(rng)
	keys, err := NewKeys(authorPriv.Key(), &readerPriv.Key().PublicKey)
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
			keys.HMACKey,
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
		AESKey:     next(x, &offset, api.AESKeyLength),
		PageIVSeed: next(x, &offset, api.PageIVSeedLength),
		HMACKey:    next(x, &offset, api.HMACKeyLength),
		MetadataIV: next(x, &offset, api.MetadataIVLength),
	}, nil
}

func next(x []byte, offset *int, len int) []byte {
	next := x[*offset : *offset+len]
	*offset += len
	return next
}
