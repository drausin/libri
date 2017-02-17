package id

import (
	"bytes"
	"crypto/ecdsa"
	crand "crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand"
)

const (
	// Length is the number of bytes in an ID, though actual representations may be shorter
	// (with the assumption that left-padded zero bytes can be omitted)
	Length = 32
)

var (
	// UpperBound is the upper bound of the ID space, i.e., all 256 bits on.
	UpperBound = FromBytes(bytes.Repeat([]byte{255}, Length))

	// LowerBound is the lower bound of the ID space, i.e., all 256 bits off.
	LowerBound = FromInt(big.NewInt(0))
)

// ID is an identifier of arbitrary byte length
type ID interface {
	// String() returns the string representation
	fmt.Stringer

	// Bytes returns the byte representation
	Bytes() []byte

	// Int returns the big.Int representation
	Int() *big.Int

	// Cmp compares the ID to another
	Cmp(ID) int

	// Distance computes the XOR distance between two IDs
	Distance(ID) *big.Int
}

// for simplicity we just store the int-representation, but we can later add and the string and/or
// bytes representations as well for speed if we want
type id struct {
	intVal *big.Int
}

func (x *id) Int() *big.Int {
	return x.intVal
}

func (x *id) Bytes() []byte {
	b := x.Int().Bytes()
	if len(b) < Length {
		lpad := make([]byte, Length-len(b))
		return append(lpad, b...)
	}
	return b
}

func (x *id) String() string {
	return fmt.Sprintf("%064X", x.Bytes())
}

func (x *id) Cmp(other ID) int {
	return x.Int().Cmp(other.Int())
}

func (x *id) Distance(y ID) *big.Int {
	return new(big.Int).Xor(x.Int(), y.Int())
}

// FromInt creates an ID from a *big.Int.
func FromInt(intVal *big.Int) ID {
	return &id{intVal: intVal}
}

// FromInt64 creates an ID from an int64.
func FromInt64(x int64) ID {
	return FromInt(big.NewInt(x))
}

// FromBytes creates an ID from a big-endian byte array.
func FromBytes(bytes []byte) ID {
	if len(bytes) > Length {
		panic(fmt.Errorf("ID byte length too long: received %v, expected <= %v", len(bytes),
			Length))
	}
	return FromInt(new(big.Int).SetBytes(bytes))
}

// NewRandomID returns a random 32-byte ID using local machine's local random number generator.
func NewRandom() ID {
	b := make([]byte, Length)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return FromBytes(b)
}

// NewPseudoRandom returns a pseudo-random ID from a random number generator.
func NewPseudoRandom(rng *mrand.Rand) ID {
	intVal := new(big.Int).Rand(rng, UpperBound.Int())
	return FromInt(intVal)
}

// FromPublicKey returns an ID instance from an elliptic curve public key.
func FromPublicKey(pubKey *ecdsa.PublicKey) ID {
	return FromInt(pubKey.X)
}
