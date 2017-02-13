package ecid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"io"
	"math/big"
	mrand "math/rand"

	cid "github.com/drausin/libri/libri/common/id"
)

var curve = elliptic.P256() // implies 32-byte private and public keys

// ID is an elliptic curve identifier, where the ID is the x-value of the (x, y) public key
// point on the curve. When couples with the private key, this allows something (e.g., a libri
// peer) to sign messages that a receiver can verify.
type ID interface {
	cid.ID

	// ECDSA private key (which includes public key as well)
	Key() *ecdsa.PrivateKey

	//
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
	key, err := ecdsa.GenerateKey(curve, reader)
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
