package id

import (
	"bytes"
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
	LowerBound = big.NewInt(0)
)

// FromBytes creates a *big.Int from a big-endian byte array.
func FromBytes(id []byte) *big.Int {
	if len(id) > Length {
		panic(fmt.Errorf("ID byte length too long: received %v, expected <= %v", len(id),
			Length))
	}
	return big.NewInt(0).SetBytes(id)
}

// NewRandomID returns a random 32-byte ID using local machine's local random number generator.
func NewRandom() *big.Int {
	b := make([]byte, Length)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return FromBytes(b)
}

// NewPseudoRandom returns a pseudo-random ID from a random number generator.
func NewPseudoRandom(rng *mrand.Rand) *big.Int {
	return big.NewInt(0).Rand(rng, UpperBound)
}

// Distance computes the XOR distance between two IDs.
func Distance(x, y *big.Int) *big.Int {
	return big.NewInt(0).Xor(x, y)
}

// String gives the string (hex) encoding of the ID.
func String(id *big.Int) string {
	return fmt.Sprintf("%064X", id.Bytes())
}
