package ecid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"errors"
	"io"
	"math/big"
	mrand "math/rand"
	"fmt"

	"github.com/drausin/libri/libri/common/id"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	cerrors "github.com/drausin/libri/libri/common/errors"
)

// Curve defines the elliptic curve public & private keys use. Curve S256 implies 32-byte private
// and 65-byte public keys, though the X value of the public key point is 32 bytes.
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
	cerrors.MaybePanic(err)  // should never happen
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

// FromPublicKeyBytes creates a new ecdsa.PublicKey from the marshaled byte representation.
func FromPublicKeyBytes(buf []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(Curve, buf) // also checks (x, y) is on curve
	if x == nil {
		return nil, ErrKeyPointOffCurve
	}
	return &ecdsa.PublicKey{
		Curve: Curve,
		X:     x,
		Y:     y,
	}, nil
}

// ToPublicKeyBytes marshals the public key of the ID to a byte representation.
func ToPublicKeyBytes(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(Curve, pub.X, pub.Y)
}
