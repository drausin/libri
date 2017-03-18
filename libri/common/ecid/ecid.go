package ecid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// Curve defines the elliptic curve public & private keys use. Curve S256 implies 32-byte private
// and 65-byte public keys, though the X value of the public key point is 32 bytes.
var Curve = secp256k1.S256()

// CurveName gives the name of the elliptic curve used for the private key.
const CurveName = "secp256k1"

// ID is an elliptic curve identifier, where the ID is the x-value of the (x, y) public key
// point on the curve. When couples with the private key, this allows something (e.g., a libri
// peer) to sign messages that a receiver can verify.
type ID interface {
	cid.ID

	// ECDSA private key (which includes public key as well)
	Key() *ecdsa.PrivateKey

	// underlying ID object
	ID() cid.ID
}

type ecid struct {
	// private & public keys + curve of this ID
	key *ecdsa.PrivateKey

	// redundant ID instance (from the public key x-value) to take advantage of existing ID
	// methods
	id cid.ID
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
	if err != nil {
		panic(err)
	}
	return &ecid{
		key: key,
		id:  cid.FromInt(key.X),
	}
}

func (x *ecid) String() string {
	return x.id.String()
}

func (x *ecid) Bytes() []byte {
	return x.id.Bytes()
}

func (x *ecid) Int() *big.Int {
	return x.id.Int()
}

func (x *ecid) Cmp(other cid.ID) int {
	return x.id.Cmp(other)
}

func (x *ecid) Distance(other cid.ID) *big.Int {
	return x.id.Distance(other)
}

func (x *ecid) Key() *ecdsa.PrivateKey {
	return x.key
}

func (x *ecid) ID() cid.ID {
	return x.id
}

// FromPublicKeyBytes creates a new ecdsa.PublicKey from the marshaled byte representation.
func FromPublicKeyBytes(buf []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(Curve, buf) // also checks (x, y) is on curve
	if x == nil {
		return nil, fmt.Errorf("unable to unmarshal bytes to point on curve %v",
			Curve.Params().Name)
	}
	return &ecdsa.PublicKey{
		Curve: Curve,
		X:     x,
		Y:     y,
	}, nil
}

// ToPublicKeyBytes marshals the public key of the ID to a byte representation.
func ToPublicKeyBytes(x ID) []byte {
	return elliptic.Marshal(Curve, x.Key().X, x.Key().Y)
}
