package ecid

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"

	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// Curve defines the elliptic curve public & private keys use. Curve S256 implies 32-byte private
// and 33-byte (compressed) public keys. The X value of the public key point is 32 bytes.
var Curve = secp256k1.S256()

// CurveName gives the name of the elliptic curve used for the private key.
const CurveName = "secp256k1"

// ErrKeyPointOffCurve indicates when a public key does not lay on the expected elliptic curve.
var ErrKeyPointOffCurve = errors.New("key point is off the expected curve")

// ID is an elliptic curve identifier, where the ID is the x-value of the (x, y) public key
// point on the curve. When coupled with the private key, this allows something (e.g., a libri
// peer) to sign messages that a receiver can verify.
type ID interface {
	fmt.Stringer

	// Bytes returns the byte representation
	Bytes() []byte

	// Int returns the big.Int representation
	Int() *big.Int

	// Cmp compares the ID to another
	Cmp(ID) int

	// Distance computes the XOR distance between two IDs
	Distance(ID) *big.Int

	// Key returns the ECDSA private key (which includes public key as well)
	Key() *ecdsa.PrivateKey

	// ID returns the underlying ID object
	ID() id.ID

	// PublicKeyBytes returns a byte slice of the encoded public key
	PublicKeyBytes() []byte
}

type ecid struct {
	// private & public keys + curve of this ID
	key *ecdsa.PrivateKey

	// redundant ID instance (from the public key x-value) to take advantage of existing ID
	// methods
	id id.ID
}

// NewRandom creates a new ID instance using a crypto.Reader source of entropy.
func NewRandom() ID {
	return newRandom(crand.Reader)
}

// NewPseudoRandom creates a new ID instance using a math.Rand source of entropy.
func NewPseudoRandom(rng *mrand.Rand) ID {
	return newRandom(rng)
}

func newRandom(reader io.Reader) ID {
	key, err := ecdsa.GenerateKey(Curve, reader)
	cerrors.MaybePanic(err) // should never happen
	return FromPrivateKey(key)
}

func (x *ecid) String() string {
	return x.id.String()
}

func (x *ecid) Bytes() []byte {
	return x.id.Bytes()
}

func (x *ecid) PublicKeyBytes() []byte {
	return ToPublicKeyBytes(&x.Key().PublicKey)
}

func (x *ecid) Int() *big.Int {
	return x.id.Int()
}

func (x *ecid) Cmp(other ID) int {
	return x.id.Cmp(other.ID())
}

func (x *ecid) Distance(other ID) *big.Int {
	return x.id.Distance(other.ID())
}

func (x *ecid) Key() *ecdsa.PrivateKey {
	return x.key
}

func (x *ecid) ID() id.ID {
	return x.id
}

// FromPrivateKey creates a new ID from an ECDSA private key.
func FromPrivateKey(priv *ecdsa.PrivateKey) ID {
	return &ecid{
		key: priv,
		id:  id.FromInt(priv.X),
	}
}

// ToPublicKeyBytes marshals the public key of the ID to the compressed byte representation.
func ToPublicKeyBytes(pub *ecdsa.PublicKey) []byte {
	return marshalCompressed(pub)
}

// FromPublicKeyBytes creates a new ecdsa.PublicKey from the marshaled compressed byte
// representation.
func FromPublicKeyBytes(buf []byte) (*ecdsa.PublicKey, error) {
	return unmarshalCompressed(buf)
}

// marshalCompressed marshals a secp256k1 public key to a compressed binary format.
// Credit:
//  - https://github.com/kmackay/micro-ecc/blob/1fce01e69c3f3c179cb9b6238391307426c5e887/
// 	  uECC.c#L1831
//  - https://github.com/fd/eccp
func marshalCompressed(pub *ecdsa.PublicKey) []byte {
	nBytes := (pub.Params().BitSize + 7) >> 3
	compressed := make([]byte, nBytes+1)
	compressed[0] = 2 + byte(pub.Y.Bit(0))
	copy(compressed[1:], pub.X.Bytes())
	return compressed
}

// unmarshalCompressed unmarshals a compressed secp256k1 point.
//
// Credit:
// 	- https://github.com/kmackay/micro-ecc/blob/1fce01e69c3f3c179cb9b6238391307426c5e887/
//	  uECC.c#L1841
//	- https://github.com/fd/eccp
func unmarshalCompressed(compressed []byte) (*ecdsa.PublicKey, error) {
	if len(compressed) != 33 {
		return nil, fmt.Errorf("unexpected compressed length: %d", len(compressed))
	}
	if compressed[0] != 2 && compressed[0] != 3 {
		return nil, fmt.Errorf("unexpected first compressed byte: %d", compressed[0])
	}

	// y^2 = x^3 + b
	x := new(big.Int).SetBytes(compressed[1:])
	lhs := new(big.Int)

	// lhs = x^2
	lhs.Mul(x, x)
	lhs.Mod(lhs, Curve.Params().P)

	// lhs = x^3
	lhs.Mul(lhs, x)
	lhs.Mod(lhs, Curve.Params().P)

	// lhs = x^3 + b
	lhs.Add(lhs, Curve.Params().B)
	lhs.Mod(lhs, Curve.Params().P)

	// lhs = sqrt(x^3 + b)
	lhs = modSqrt(lhs)

	// ensure correct of two possible y points
	if lhs.Bit(0) != uint(compressed[0]&0x01) {
		lhs.Sub(Curve.Params().P, lhs)
	}
	y := lhs

	if !Curve.IsOnCurve(x, y) {
		return nil, ErrKeyPointOffCurve
	}
	return &ecdsa.PublicKey{
		Curve: Curve,
		X:     x,
		Y:     y,
	}, nil
}

// Credit:
// 	- https://github.com/kmackay/micro-ecc/blob/1fce01e69c3f3c179cb9b6238391307426c5e887/
//    uECC.c#L1685
//  - https://github.com/fd/eccp
func modSqrt(a *big.Int) *big.Int {
	p1 := big.NewInt(1)
	p1.Add(p1, Curve.Params().P)

	result := big.NewInt(1)

	for i := p1.BitLen() - 1; i > 1; i-- {
		result.Mul(result, result)
		result.Mod(result, Curve.Params().P)
		if p1.Bit(i) > 0 {
			result.Mul(result, a)
			result.Mod(result, Curve.Params().P)
		}
	}

	return result
}
