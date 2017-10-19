package id

import (
	"bytes"
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	mrand "math/rand"

	"github.com/drausin/libri/libri/common/errors"
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
	return Hex(x.Bytes())
}

func (x *id) Cmp(other ID) int {
	return x.Int().Cmp(other.Int())
}

func (x *id) Distance(y ID) *big.Int {
	return new(big.Int).Xor(x.Int(), y.Int())
}

// FromInt creates an ID from a *big.Int.
func FromInt(value *big.Int) ID {
	return &id{intVal: value}
}

// FromInt64 creates an ID from an int64.
func FromInt64(value int64) ID {
	return FromInt(big.NewInt(value))
}

// FromBytes creates an ID from a big-endian byte array.
func FromBytes(value []byte) ID {
	if len(value) > Length {
		panic(fmt.Errorf("ID byte length too long: received %v, expected <= %v", len(value),
			Length))
	}
	return FromInt(new(big.Int).SetBytes(value))
}

// FromString creates an ID from a hex-encoded string.
func FromString(value string) (ID, error) {
	if len(value) != Length*2 {
		return nil, fmt.Errorf("%s (len: %d) is not a valid %d-byte hex string", value,
			len(value), Length)
	}
	bytesID, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return FromBytes(bytesID), nil
}

// NewRandom returns a random 32-byte ID using local machine's local random number generator.
func NewRandom() ID {
	b := make([]byte, Length)
	_, err := crand.Read(b)
	errors.MaybePanic(err)
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

// Hex returns the 64-char hex value of a 32-byte values.
func Hex(val []byte) string {
	format := "%064x"
	if len(val) > 32 {
		return fmt.Sprintf(format, val[:32])
	}
	return fmt.Sprintf("%064x", val)
}

// ShortHex returns the 16-char hex value of the first 8 butes of a value.
func ShortHex(val []byte) string {
	format := "%016x"
	if len(val) > 8 {
		return fmt.Sprintf(format, val[:8])
	}
	return fmt.Sprintf(format, val)
}
