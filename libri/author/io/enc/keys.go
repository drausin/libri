package enc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"math/rand"

	"errors"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/crypto/hkdf"
)

// ErrAuthorOffCurve indicates that an author ECDSA private key is is not on the expected curve.
var ErrAuthorOffCurve = errors.New("author public key point not on expected elliptic curve")

// ErrReaderOffCurve indicates that a reader ECDSA public key is is not on the expected curve.
var ErrReaderOffCurve = errors.New("reader public key point not on expected elliptic curve")

// ErrIncompleteKeyDefinition indicates that the key definition function was unable to generate
// the required number of bytes for the encryption keys.
var ErrIncompleteKeyDefinition = errors.New("incomplete key definition")

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

// NewKeys generates a *Keys instance using the private and public ECDSA keys.
func NewKeys(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey) (*Keys, error) {
	if !ecid.Curve.IsOnCurve(priv.X, priv.Y) {
		return nil, ErrAuthorOffCurve
	}
	if !ecid.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, ErrReaderOffCurve
	}
	secretX, _ := ecid.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	kdf := hkdf.New(sha256.New, secretX.Bytes(), nil, nil)
	keyBytes := make([]byte, api.EncryptionKeysLength)
	n, err := kdf.Read(keyBytes)
	if err != nil {
		return nil, err
	}
	if n != api.EncryptionKeysLength {
		return nil, ErrIncompleteKeyDefinition
	}
	return Unmarshal(keyBytes)
}

// NewPseudoRandomKeys generates a new *Keys instance from the shared secret between a random
// ECDSA private and separate ECDSA public key. It also returns the author and reader public keys,
// serialized to byte slices.
func NewPseudoRandomKeys(rng *rand.Rand) (*Keys, []byte, []byte) {
	authorPriv := ecid.NewPseudoRandom(rng)
	readerPriv := ecid.NewPseudoRandom(rng)
	keys, err := NewKeys(authorPriv.Key(), &readerPriv.Key().PublicKey)
	if err != nil {
		panic(err)
	}
	return keys, ecid.ToPublicKeyBytes(authorPriv), ecid.ToPublicKeyBytes(readerPriv)
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
