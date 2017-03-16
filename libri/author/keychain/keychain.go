package keychain

import (
	"crypto/ecdsa"
	"github.com/drausin/libri/libri/common/ecid"
	"io/ioutil"
	"github.com/golang/protobuf/proto"
)



// Keychain represents a collection of ECDSA private keys.
type Keychain struct {
	// keys indexed by the hex string of the public key X value (a.k.a., ecid.ID.String())
	keyEncKeys map[string]*ecdsa.PrivateKey
}


func New(n int) *Keychain {
	keys := make(map[string]*ecdsa.PrivateKey)
	for c := 0; c < n; c++ {
		ecidKey := ecid.NewRandom()
		keys[ecidKey.String()] = ecidKey.Key()
	}
	return &Keychain{keys}
}

func Load(filepath, auth string) (*Keychain, error) {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	stored := &StoredKeychain{}
	if err := proto.Unmarshal(buf, stored); err != nil {
		return nil, err
	}
	return FromStored(stored, auth)
}

func Save(filepath, auth string, kc *Keychain, scryptN, scryptP int) error {
	stored, err := ToStored(kc, auth, scryptN, scryptP)
	if err != nil {
		return err
	}
	buf, err := proto.Marshal(stored)
	if err != nil {
		return err
	}
	const filePerm = 0600  // only user can read
	ioutil.WriteFile(filepath, buf, filePerm)
	return nil
}
