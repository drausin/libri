package author

import (
	"os"
	"path"

	"errors"

	"fmt"

	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

var (
	// ErrKeychainExists indicates when a keychain file already exists.
	ErrKeychainExists = errors.New("keychain already exists")

	errMissingAuthorKeychain = fmt.Errorf("missing %s, but have %s", AuthorKeychainFilename,
		SelfReaderKeychainFilename)

	errMissingSelfReaderKeychain = fmt.Errorf("missing %s, but have %s",
		SelfReaderKeychainFilename, AuthorKeychainFilename)
)

// logger keys
const (
	// LoggerClientID is a client ID.
	LoggerClientID = "clientId"

	// LoggerKeychainFilepath is a keychain filepath.
	LoggerKeychainFilepath = "keychainFilepath"

	// LoggerKeychainNKeys is the number of keys in the keychain.
	LoggerKeychainNKeys = "nKeys"
)

const (
	// AuthorKeychainFilename defines the author keychain filename
	AuthorKeychainFilename = "author.keys"

	// SelfReaderKeychainFilename defines the self reader keychain filename
	SelfReaderKeychainFilename = "self-reader.keys"

	// nInitialKeys is the number of keys to generate on a keychain
	nInitialKeys = 64
)

var (
	clientIDKey = []byte("ClientID")
)

func loadOrCreateClientID(logger *zap.Logger, nsl storage.NamespaceSL) (ecid.ID, error) {
	bytes, err := nsl.Load(clientIDKey)
	if err != nil {
		logger.Error("error loading client ID", zap.Error(err))
		return nil, err
	}

	if bytes != nil {
		// return saved PeerID
		stored := &ecid.ECDSAPrivateKey{}
		if err := proto.Unmarshal(bytes, stored); err != nil {
			return nil, err
		}
		clientID, err := ecid.FromStored(stored)
		if err != nil {
			logger.Error("error deserializing client ID keys", zap.Error(err))
			return nil, err
		}
		logger.Info("loaded exsting client ID", zap.String(LoggerClientID,
			clientID.String()))
		return clientID, nil
	}

	// return new client ID
	clientID := ecid.NewRandom()
	logger.Info("created new client ID", zap.String(LoggerClientID, clientID.String()))

	return clientID, saveClientID(nsl, clientID)
}

func saveClientID(ns storage.NamespaceStorer, clientID ecid.ID) error {
	bytes, err := proto.Marshal(ecid.ToStored(clientID))
	if err != nil {
		return err
	}
	return ns.Store(clientIDKey, bytes)
}

// LoadKeychains loads the author and self-reader keychains from a directory on the local
// filesystem.
func LoadKeychains(keychainDir, auth string) (
	keychain.GetterSampler, keychain.GetterSampler, error) {

	authorKeychainFilepath := path.Join(keychainDir, AuthorKeychainFilename)
	authorKeys, err := keychain.Load(authorKeychainFilepath, auth)
	if err != nil {
		return nil, nil, err
	}

	selfReaderKeychainPath := path.Join(keychainDir, SelfReaderKeychainFilename)
	selfReaderKeys, err := keychain.Load(selfReaderKeychainPath, auth)
	if err != nil {
		return nil, nil, err
	}

	return authorKeys, selfReaderKeys, nil
}

// CreateKeychains creates the author and self reader keychains in the given keychain directory with
// the given authentication passphrase and Scrypt parameters.
func CreateKeychains(logger *zap.Logger, keychainDir, auth string, scryptN, scryptP int) error {
	if _, err := os.Stat(keychainDir); os.IsNotExist(err) {
		err := os.MkdirAll(keychainDir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	authorKeychainFP := path.Join(keychainDir, AuthorKeychainFilename)
	if err := CreateKeychain(logger, authorKeychainFP, auth, scryptN, scryptP); err != nil {
		return err
	}
	selfReaderKeysFP := path.Join(keychainDir, SelfReaderKeychainFilename)
	return CreateKeychain(logger, selfReaderKeysFP, auth, scryptN, scryptP)
}

// CreateKeychain creates a keychain in the given filepath with the given auth and Scrypt params.
func CreateKeychain(logger *zap.Logger, filepath, auth string, scryptN, scryptP int) error {
	if info, _ := os.Stat(filepath); info != nil {
		logger.Error("keychain already exists",
			zap.String(LoggerKeychainFilepath, filepath))
		return ErrKeychainExists
	}

	keys := keychain.New(nInitialKeys)
	err := keychain.Save(filepath, auth, keys, scryptN, scryptP)
	if err != nil {
		return err
	}
	logger.Info("saved new keychain", zap.String(LoggerKeychainFilepath, filepath),
		zap.Int(LoggerKeychainNKeys, nInitialKeys))
	return nil
}

// MissingKeychains determines whether the author and self-reader keychains are missing.
func MissingKeychains(keychainDir string) (bool, error) {
	if _, err := os.Stat(keychainDir); os.IsNotExist(err) {
		return true, nil
	}
	authorFileInfo, _ := os.Stat(path.Join(keychainDir, AuthorKeychainFilename))
	selfReaderFileInfo, _ := os.Stat(path.Join(keychainDir, SelfReaderKeychainFilename))

	if authorFileInfo == nil && selfReaderFileInfo == nil {
		// neither file exists
		return true, nil
	}

	if authorFileInfo != nil && selfReaderFileInfo != nil {
		// both files exist
		return false, nil
	}

	if authorFileInfo == nil {
		return true, errMissingAuthorKeychain
	}

	if selfReaderFileInfo == nil {
		return true, errMissingSelfReaderKeychain
	}

	panic(errors.New("should never get here"))
}
