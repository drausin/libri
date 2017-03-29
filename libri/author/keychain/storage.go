package keychain

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	ethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
)

// EncryptToStored encrypts the contents of Keychain using the authentication passphrase and scrypt
// difficulty parameters.
func EncryptToStored(kc *Keychain, auth string, scryptN, scryptP int) (*StoredKeychain, error) {
	storedKeyEncKeys := make(map[string][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs, done := make(chan error, 1), make(chan struct{}, 1)

	// encrypt all keys in parallel b/c each can be intensive, thanks to scrypt
	for s1, key1 := range kc.keyEncKeys {
		wg.Add(1)
		go func(s2 string, key2 *ecdsa.PrivateKey) {
			var err error
			defer wg.Done()
			mu.Lock()
			storedKeyEncKeys[s2], err = encryptKey(key2, auth, scryptN, scryptP)
			mu.Unlock()
			if err != nil {
				errs <- err
			}
		}(s1, key1)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return &StoredKeychain{
			KeyEncKeys: storedKeyEncKeys,
		}, nil
	case err := <-errs:
		return nil, err
	}
}

// DecryptFromStored decrypts the contents of a StoredKeychain using the authentication passphrase.
func DecryptFromStored(stored *StoredKeychain, auth string) (*Keychain, error) {
	keyEncKeys := make(map[string]*ecdsa.PrivateKey)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs, done := make(chan error, 1), make(chan struct{}, 1)

	// decrypt all keys in parallel b/c each can be intensive, thanks to scrypt
	for _, keyJSON1 := range stored.KeyEncKeys {
		wg.Add(1)
		go func(keyJSON2 []byte) {
			defer wg.Done()
			keyEncKey, err := decryptKey(keyJSON2, auth)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			keyEncKeys[pubKeyString(keyEncKey)] = keyEncKey
			mu.Unlock()
		}(keyJSON1)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return &Keychain{
			keyEncKeys: keyEncKeys,
		}, nil
	case err := <-errs:
		return nil, err
	}
}

func encryptKey(key *ecdsa.PrivateKey, auth string, scryptN, scryptP int) ([]byte, error) {
	ethKey := &ethkeystore.Key{
		// Address is not not used by libri, but required for encryption & decryption
		Address:    crypto.PubkeyToAddress(key.PublicKey),
		PrivateKey: key,
	}
	return ethkeystore.EncryptKey(ethKey, auth, scryptN, scryptP)
}

func decryptKey(keyJSON []byte, auth string) (*ecdsa.PrivateKey, error) {
	ethKey, err := ethkeystore.DecryptKey(keyJSON, auth)
	if err != nil {
		return nil, err
	}
	return ethKey.PrivateKey, nil
}

func pubKeyString(privKey *ecdsa.PrivateKey) string {
	return fmt.Sprintf("%064x", privKey.X.Bytes())
}
