package keychain

import (
	"crypto/ecdsa"
	"sync"

	ethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/drausin/libri/libri/common/ecid"
)

// encryptToStored encrypts the contents of Keychain using the authentication passphrase and scrypt
// difficulty parameters.
func encryptToStored(kc Keychain, auth string, scryptN, scryptP int) (*StoredKeychain, error) {
	storedPrivateKeys := make([][]byte, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs, done := make(chan error, 1), make(chan struct{}, 1)

	// encrypt all keys in parallel b/c each can be intensive, thanks to scrypt
	for _, key1 := range kc.(*keychain).privs {
		wg.Add(1)
		go func(key2 ecid.ID) {
			var err error
			defer wg.Done()
			encryptedKeyBytes, err := encryptKey(key2.Key(), auth, scryptN, scryptP)
			if err != nil {
				errs <- err
			}
			mu.Lock()
			storedPrivateKeys = append(storedPrivateKeys, encryptedKeyBytes)
			mu.Unlock()
		}(key1)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return &StoredKeychain{
			PrivateKeys: storedPrivateKeys,
		}, nil
	case err := <-errs:
		return nil, err
	}
}

// decryptFromStored decrypts the contents of a StoredKeychain using the authentication passphrase.
func decryptFromStored(stored *StoredKeychain, auth string) (Keychain, error) {
	ecids := make([]ecid.ID, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs, done := make(chan error, 1), make(chan struct{}, 1)

	// decrypt all keys in parallel b/c each can be intensive, thanks to scrypt
	for _, keyJSON1 := range stored.PrivateKeys {
		wg.Add(1)
		go func(keyJSON2 []byte) {
			defer wg.Done()
			priv, err := decryptKey(keyJSON2, auth)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			ecids = append(ecids, ecid.FromPrivateKey(priv))
			mu.Unlock()
		}(keyJSON1)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return FromECIDs(ecids), nil
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
