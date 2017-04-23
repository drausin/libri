package keychain

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"sort"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/golang/protobuf/proto"
)

const (
	// StandardScryptN is the N parameter of Scrypt encryption algorithm, using 256MB
	// memory and taking approximately 1s CPU time on a modern processor.
	StandardScryptN = 1 << 18

	// StandardScryptP is the P parameter of Scrypt encryption algorithm, using 256MB
	// memory and taking approximately 1s CPU time on a modern processor.
	StandardScryptP = 1

	// LightScryptN is the N parameter of Scrypt encryption algorithm, using 4MB
	// memory and taking approximately 100ms CPU time on a modern processor.
	LightScryptN = 1 << 12

	// LightScryptP is the P parameter of Scrypt encryption algorithm, using 4MB
	// memory and taking approximately 100ms CPU time on a modern processor.
	LightScryptP = 6
)

var (
	// ErrEmptyKeychain indicates no keys in the keychain.
	ErrEmptyKeychain = errorsNew("empty keychain")

	// ErrUnexpectedMissingKey indicates a unexpectedly missing key
	ErrUnexpectedMissingKey = errorsNew("missing key")
)

// Keychain is a collection of ECDSA keys.
type Keychain interface {
	// Sample randomly selects a key.
	Sample() (ecid.ID, error)

	// Get returns the key with the given public key, if it exists. Otherwise, it returns nil.
	// The second return value indicates whether the key is present in the keychain or not.
	Get(publicKey []byte) (ecid.ID, bool)

	// Len returns the number of keys in the keychain.
	Len() int
}

// Keychain represents a collection of ECDSA private keys.
type keychain struct {
	// private keys indexed by the hex of the 65-byte public key representation
	privs map[string]ecid.ID

	// hex 65-byte public key representations
	pubs []string

	// random number generator for sampling keys
	rng *rand.Rand
}

// New creates a new (plaintext) Keychain with n individual keys.
func New(n int) Keychain {
	ecids := make([]ecid.ID, n)
	for i := 0; i < n; i++ {
		ecids[i] = ecid.NewRandom()
	}
	return FromECIDs(ecids)
}

// FromECIDs creates a Keychain instance from a map of ECDSA private keys.
func FromECIDs(ecids []ecid.ID) Keychain {
	pubs := make([]string, len(ecids))
	privs := make(map[string]ecid.ID)
	for i, priv := range ecids {
		pubs[i] = pubKeyString(ecid.ToPublicKeyBytes(priv))
		privs[pubs[i]] = priv
	}
	sort.Strings(pubs)
	return &keychain{
		privs: privs,
		pubs:  pubs,
		rng:   rand.New(rand.NewSource(int64(len(privs)))),
	}
}

// Sample returns a uniformly random key from the keychain.
func (kc *keychain) Sample() (ecid.ID, error) {
	if len(kc.pubs) == 0 {
		return nil, ErrEmptyKeychain
	}
	i := kc.rng.Int31n(int32(len(kc.pubs)))
	return kc.privs[kc.pubs[i]], nil
}

func (kc *keychain) Get(publicKey []byte) (ecid.ID, bool) {
	value, in := kc.privs[pubKeyString(publicKey)]
	return value, in
}

func (kc *keychain) Len() int {
	return len(kc.pubs)
}

// Save saves and encrypts a keychain to a file.
func Save(filepath, auth string, kc Keychain, scryptN, scryptP int) error {
	stored, err := encryptToStored(kc, auth, scryptN, scryptP)
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
func Load(filepath, auth string) (Keychain, error) {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	stored := &StoredKeychain{}
	if err := proto.Unmarshal(buf, stored); err != nil {
		return nil, err
	}
	return decryptFromStored(stored, auth)
}

func pubKeyString(pubKey []byte) string {
	return fmt.Sprintf("%065x", pubKey)
}
