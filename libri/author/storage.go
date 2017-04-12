package author

import (
	"go.uber.org/zap"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/golang/protobuf/proto"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/author/keychain"
	"path"
	"os"
	"github.com/pkg/errors"
)

// ErrKeychainExists indicates when a keychain file already exists.
var ErrKeychainExists = errors.New("keychain already exists")

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
	// authorKeychainFilename defines the author keychain filename
	authorKeychainFilename = "author.keys"

	// selfReaderKeychainFilename defines the self reader keychain filename
	selfReaderKeychainFilename = "self-reader.keys"

	// nInitialKeys is the number of keys to generate on a keychain
	nInitialKeys = 64
)

var (
	clientIDKey = []byte("ClientID")
)

func loadOrCreateClientID(logger *zap.Logger, nsl storage.NamespaceStorerLoader) (ecid.ID, error) {
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

func loadKeychains(keychainDir, auth string) (keychain.Keychain, keychain.Keychain, error) {
	authorKeychainFilepath := path.Join(keychainDir, authorKeychainFilename)
	authorKeys, err := keychain.Load(authorKeychainFilepath, auth)
	if err != nil {
		return nil, nil, err
	}

	selfReaderKeychainPath := path.Join(keychainDir, selfReaderKeychainFilename)
	selfReaderKeys, err := keychain.Load(selfReaderKeychainPath, auth)
	if err != nil {
		return nil, nil, err
	}

	return authorKeys, selfReaderKeys, nil
}

func createKeychains(logger *zap.Logger, keychainDir, auth string, scryptN, scryptP int) error {
	if _, err := os.Stat(keychainDir); os.IsNotExist(err) {
		err := os.MkdirAll(keychainDir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	authorKeychainFP := path.Join(keychainDir, authorKeychainFilename)
	if err := createKeychain(logger, authorKeychainFP, auth, scryptN, scryptP); err != nil {
		return err
	}
	selfReaderKeysFP := path.Join(keychainDir, selfReaderKeychainFilename)
	if err := createKeychain(logger, selfReaderKeysFP, auth, scryptN, scryptP); err != nil {
		return err
	}
	return nil
}

func createKeychain(logger *zap.Logger, filepath, auth string, scryptN, scryptP int) error {
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

