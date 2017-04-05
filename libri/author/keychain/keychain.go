package keychain

import (
	"crypto/ecdsa"
	"io/ioutil"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/golang/protobuf/proto"
	"math/rand"
)

// Sampler randomly selects a key (in the form of an ecid.ID) from a keychain.
type Sampler interface {
	// Sample randomly selects a key from the keychain.
	Sample() (ecid.ID, error)
}

// Keychain represents a collection of ECDSA private keys.
type Keychain struct {
	// private keys indexed by the hex string of the public key X value
	// (a.k.a., ecid.ID.String())
	privs map[string]*ecdsa.PrivateKey

	// hex string of public key X values
	pubs []string

	// random number generator for sampling keys
	rng   *rand.Rand
}

// New creates a new (plaintext) Keychain with n individual keys.
func New(n int) *Keychain {
	privs := make(map[string]*ecdsa.PrivateKey)
	for i := 0; i < n; i++ {
		ecidKey := ecid.NewRandom()
		privs[ecidKey.String()] = ecidKey.Key()
	}
	return FromPrivateKeys(privs)
}

// FromPrivateKeys creates a *Keychain instance from a map of ECDSA private keys.
func FromPrivateKeys(privs map[string]*ecdsa.PrivateKey) *Keychain {
	pubs, i := make([]string, len(privs)), 0
	for pub := range privs {
		pubs[i] = pub
		i++
	}
	return &Keychain{
		privs: privs,
		pubs: pubs,
		rng: rand.New(rand.NewSource(int64(len(privs)))),
	}
}

func (kc *Keychain) Sample() ecid.ID {
	i := kc.rng.Int31n(int32(len(kc.pubs)))
	priv := kc.privs[kc.pubs[i]]
	return ecid.FromPrivateKey(priv)
}

// Save saves and encrypts a keychain to a file.
func Save(filepath, auth string, kc *Keychain, scryptN, scryptP int) error {
	stored, err := EncryptToStored(kc, auth, scryptN, scryptP)
	if err != nil {
		return err
	}
	buf, err := proto.Marshal(stored)
	if err != nil {
		return err
	}
	const filePerm = 0600 // only user can read
	return ioutil.WriteFile(filepath, buf, filePerm)
}

// Load loads and decrypts a keychain from a file.
func Load(filepath, auth string) (*Keychain, error) {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	stored := &StoredKeychain{}
	if err := proto.Unmarshal(buf, stored); err != nil {
		return nil, err
	}
	return DecryptFromStored(stored, auth)
}
