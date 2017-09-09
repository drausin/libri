package enc

import (
	"bytes"
	"crypto/aes"
	"crypto/ecdsa"
	crand "crypto/rand"
	"crypto/sha256"
	"errors"
	mrand "math/rand"

	"crypto/cipher"

	"github.com/drausin/libri/libri/common/ecid"
	cerrors "github.com/drausin/libri/libri/common/errors"
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

// ErrInsufficientEEKBytes indicates when the crypto random number generator is unable to generate
// the sufficient number of bytes for the EEK key.
var ErrInsufficientEEKBytes = errors.New("insufficient EEK bytes")

// KEK (key encryption keys) are used to encrypt an EEK.
type KEK struct {
	// AESKey is the 32-byte AES-256 key used to encrypt the EEK.
	AESKey []byte

	// IV is the 32-byte block cipher initialization vector (IV) seed.
	IV []byte

	// HMACKey is the 32-byte key used for the EEK ciphertext HMAC-256.
	HMACKey []byte
}

// NewKEK generates a new *KEK instance from the shared secret between a random ECDSA private and
// separate ECDSA public key. It also returns the author and reader public keys, serialized to
// byte slices.
func NewKEK(priv *ecdsa.PrivateKey, pub *ecdsa.PublicKey) (*KEK, error) {
	if !ecid.Curve.IsOnCurve(priv.X, priv.Y) {
		return nil, ErrAuthorOffCurve
	}
	if !ecid.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, ErrReaderOffCurve
	}
	secretX, _ := ecid.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	kdf := hkdf.New(sha256.New, secretX.Bytes(), nil, nil)
	kekBytes := make([]byte, api.KEKLength)
	n, err := kdf.Read(kekBytes)
	if err != nil {
		return nil, err
	}
	if n != api.KEKLength {
		return nil, ErrIncompleteKeyDefinition
	}
	return UnmarshalKEK(kekBytes)
}

// NewPseudoRandomKEK generates a new *KEK instance from a random number generator for use in
// testing.
func NewPseudoRandomKEK(rng *mrand.Rand) (*KEK, []byte, []byte) {
	authorPriv := ecid.NewPseudoRandom(rng)
	readerPriv := ecid.NewPseudoRandom(rng)
	keys, err := NewKEK(authorPriv.Key(), &readerPriv.Key().PublicKey)
	cerrors.MaybePanic(err)
	return keys, authorPriv.PublicKeyBytes(), readerPriv.PublicKeyBytes()
}

// Encrypt encrypts the EEK, returning the EEK ciphertext and ciphertext MAC.
func (kek *KEK) Encrypt(eek *EEK) ([]byte, []byte, error) {
	gcmCipher, err := newGCMCipher(kek.AESKey)
	if err != nil {
		return nil, nil, err
	}
	plaintext := MarshalEEK(eek)
	eekCiphertext := gcmCipher.Seal(nil, kek.IV, plaintext, nil)
	eekCiphertextMAC := HMAC(eekCiphertext, kek.HMACKey)
	return eekCiphertext, eekCiphertextMAC, nil
}

// Decrypt checkes the EEK ciphertext MAC and decrypts it to a an EEK.
func (kek *KEK) Decrypt(eekCiphertext []byte, eekCiphertextMAC []byte) (*EEK, error) {
	gcmCipher, err := newGCMCipher(kek.AESKey)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(eekCiphertextMAC, HMAC(eekCiphertext, kek.HMACKey)) {
		return nil, ErrUnexpectedCiphertextMAC
	}
	eekPlaintext, err := gcmCipher.Open(nil, kek.IV, eekCiphertext, nil)
	if err != nil {
		return nil, err
	}
	return UnmarshalEEK(eekPlaintext)
}

// MarshalKEK serializes the EEK to their byte representation.
func MarshalKEK(keys *KEK) []byte {
	return bytes.Join(
		[][]byte{
			keys.AESKey,
			keys.IV,
			keys.HMACKey,
		},
		[]byte{},
	)
}

// UnmarshalKEK deserializes the KEK from its byte representation.
func UnmarshalKEK(x []byte) (*KEK, error) {
	err := api.ValidateBytes(x, api.KEKLength, "KEK byte representation")
	if err != nil {
		return nil, err
	}
	var offset int
	return &KEK{
		AESKey:  next(x, &offset, api.AESKeyLength),
		IV:      next(x, &offset, api.BlockCipherIVLength),
		HMACKey: next(x, &offset, api.HMACKeyLength),
	}, nil
}

// EEK (entry encryption keys) are used to encrypt an Entry and its Pages.
type EEK struct {
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

// NewEEK generates a *EEK instance using the private and public ECDSA keys.
func NewEEK() (*EEK, error) {
	eekBytes := make([]byte, api.EEKLength)
	nRead1, err := crand.Reader.Read(eekBytes)
	if err != nil {
		return nil, err
	}
	if nRead1 != api.EEKLength {
		// sometimes the crand.Reader runs out of bytes, so try reading again one more time
		nRead2, err := crand.Reader.Read(eekBytes)
		if err != nil {
			return nil, err
		}
		if nRead2 != api.EEKLength {
			return nil, ErrInsufficientEEKBytes
		}
	}
	return UnmarshalEEK(eekBytes)
}

// NewPseudoRandomEEK generates a new *EEK instance from a random number generator for use in
// testing.
func NewPseudoRandomEEK(rng *mrand.Rand) *EEK {
	eekBytes := make([]byte, api.EEKLength)
	nRead, err := rng.Read(eekBytes)
	cerrors.MaybePanic(err)
	if nRead != api.EEKLength {
		panic(err)
	}
	eek, err := UnmarshalEEK(eekBytes)
	cerrors.MaybePanic(err)
	return eek
}

// MarshalEEK serializes the EEK to their byte representation.
func MarshalEEK(keys *EEK) []byte {
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

// UnmarshalEEK deserializes the EEK from its byte representation.
func UnmarshalEEK(x []byte) (*EEK, error) {
	err := api.ValidateBytes(x, api.EEKLength, "EEK byte representation")
	if err != nil {
		return nil, err
	}
	var offset int
	return &EEK{
		AESKey:     next(x, &offset, api.AESKeyLength),
		PageIVSeed: next(x, &offset, api.PageIVSeedLength),
		HMACKey:    next(x, &offset, api.HMACKeyLength),
		MetadataIV: next(x, &offset, api.BlockCipherIVLength),
	}, nil
}

func newGCMCipher(aesKey []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func next(x []byte, offset *int, len int) []byte {
	next := x[*offset : *offset+len]
	*offset += len
	return next
}
