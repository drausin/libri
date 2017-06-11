package storage

import (
	"github.com/drausin/libri/libri/common/db"
)

const (
	// MaxNamespaceLength is the max (byte) length of a namespace.
	MaxNamespaceLength = 256
)

// Storer stores a value to durable storage.
type Storer interface {
	// Store a key-value pair in a given namespace.
	Store(namespace []byte, key []byte, value []byte) error
}

// Loader loads a value from durable storage.
type Loader interface {
	// Load a value for a given key and namespace.
	Load(namespace []byte, key []byte) ([]byte, error)
}

// Deleter deletes a value from durable storage.
type Deleter interface {
	// Delete a value with the given key and namespace.
	Delete(namespace []byte, key []byte) error
}

// StorerLoader can both store and load values.
type StorerLoader interface {
	Storer
	Loader
}

// StorerLoaderDeleter can store, load, and delete values.
type StorerLoaderDeleter interface {
	StorerLoader
	Deleter
}

type kvdbSLD struct {
	db db.KVDB
	nc Checker
	kc Checker
	vc Checker
}

// NewKVDBStorerLoaderDeleter returns a new StorerLoaderDeleter backed by a db.KVDB instance and
// with the given key and value checkers.
func NewKVDBStorerLoaderDeleter(
	db db.KVDB,
	keyChecker Checker,
	valueChecker Checker,
) StorerLoaderDeleter {
	return &kvdbSLD{
		db: db,
		nc: NewMaxLengthChecker(MaxNamespaceLength),
		kc: keyChecker,
		vc: valueChecker,
	}
}

func NewKVDBStorerLoader(db db.KVDB, keyChecker Checker, valueChecker Checker) StorerLoader {
	return NewKVDBStorerLoaderDeleter(db, keyChecker, valueChecker)
}

func (sld *kvdbSLD) Store(namespace []byte, key []byte, value []byte) error {
	if err := sld.nc.Check(namespace); err != nil {
		return err
	}
	if err := sld.kc.Check(key); err != nil {
		return err
	}
	if err := sld.vc.Check(value); err != nil {
		return err
	}
	return sld.db.Put(namespaceKey(namespace, key), value)
}

func (sld *kvdbSLD) Load(namespace []byte, key []byte) ([]byte, error) {
	if err := sld.nc.Check(namespace); err != nil {
		return nil, err
	}
	if err := sld.kc.Check(key); err != nil {
		return nil, err
	}
	return sld.db.Get(namespaceKey(namespace, key))
}

func (sld *kvdbSLD) Delete(namespace []byte, key []byte) error {
	if err := sld.nc.Check(namespace); err != nil {
		return err
	}
	if err := sld.kc.Check(key); err != nil {
		return err
	}
	return sld.db.Delete(namespaceKey(namespace, key))
}

func namespaceKey(namespace []byte, key []byte) []byte {
	return append(namespace, key...)
}
