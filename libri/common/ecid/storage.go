package ecid

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// FromStored creates a new ID instance from a ECID instance.
func FromStored(stored *ECDSAPrivateKey) (ID, error) {
	key := new(ecdsa.PrivateKey)

	switch stored.Curve {
	case "secp256k1":
		key.PublicKey.Curve = secp256k1.S256()
	default:
		return nil, fmt.Errorf("unrecognized curve %v", stored.Curve)
	}

	key.PublicKey.X = new(big.Int).SetBytes(stored.X)
	key.PublicKey.Y = new(big.Int).SetBytes(stored.Y)
	key.D = new(big.Int).SetBytes(stored.D)

	if !key.Curve.IsOnCurve(key.PublicKey.X, key.PublicKey.Y) {
		// redundancy check: should never hit this, but here just in case
		return nil, fmt.Errorf("public key (x = %v, y = %v) is not on curve %v",
			key.PublicKey.X, key.PublicKey.Y, key.PublicKey.Curve.Params().Name)
	}

	return &ecid{
		key: key,
		id:  cid.FromInt(key.X),
	}, nil
}

// ToStored creates a new ECID instance from an ID instance.
func ToStored(ecid ID) *ECDSAPrivateKey {
	key := ecid.Key()
	return &ECDSAPrivateKey{
		Curve: CurveName,
		X:     key.X.Bytes(),
		Y:     key.Y.Bytes(),
		D:     key.D.Bytes(),
	}
}
