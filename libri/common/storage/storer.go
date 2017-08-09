package storage

import (
	"github.com/drausin/libri/libri/common/db"
)

// Storer stores a value to durable storage.
type Storer interface {
	// Store a key-value pair in a given namespace.
	Store(key []byte, value []byte) error

	// Iterate iterates through the key-value pairs of a given namespace within a given key range.
	// It calls the callback function for each key-value pair.
	Iterate(keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte)) error
}

// Loader loads a value from durable storage.
type Loader interface {
	// Load a value for a given key and namespace.
	Load(key []byte) ([]byte, error)
}

// Deleter deletes a value from durable storage.
type Deleter interface {
	// Delete a value with the given key and namespace.
	Delete(key []byte) error
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
	ns []byte
	db db.KVDB
	kc Checker
	vc Checker
}

// NewKVDBStorerLoaderDeleter returns a new StorerLoaderDeleter backed by a db.KVDB instance and
// with the given key and value checkers.
func NewKVDBStorerLoaderDeleter(
	ns []byte, db db.KVDB, keyChecker Checker, valueChecker Checker,
) StorerLoaderDeleter {
	return &kvdbSLD{
		ns: ns,
		db: db,
		kc: keyChecker,
		vc: valueChecker,
	}
}

// NewKVDBStorerLoader returns a new StorerLoader backed by a db.KVDB instance and with the given
// key and value checkers.
func NewKVDBStorerLoader(
	ns []byte, db db.KVDB, keyChecker Checker, valueChecker Checker,
) StorerLoader {
	return NewKVDBStorerLoaderDeleter(ns, db, keyChecker, valueChecker)
}

func (sld *kvdbSLD) Store(key, value []byte) error {
	if err := sld.kc.Check(key); err != nil {
		return err
	}
	if err := sld.vc.Check(value); err != nil {
		return err
	}
	return sld.db.Put(namespaceKey(sld.ns, key), value)
}

func (sld *kvdbSLD) Iterate(
	keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte),
) error {
	lb, ub := namespaceKey(sld.ns, keyLB), namespaceKey(sld.ns, keyUB)
	return sld.db.Iterate(lb, ub, done, func(nsKey, value []byte) {
		key := nsKey[len(sld.ns):]
		callback(key, value)
	})
}

func (sld *kvdbSLD) Load(key []byte) ([]byte, error) {
	if err := sld.kc.Check(key); err != nil {
		return nil, err
	}
	return sld.db.Get(namespaceKey(sld.ns, key))
}

func (sld *kvdbSLD) Delete(key []byte) error {
	if err := sld.kc.Check(key); err != nil {
		return err
	}
	return sld.db.Delete(namespaceKey(sld.ns, key))
}

func namespaceKey(namespace []byte, key []byte) []byte {
	nsKey := make([]byte, len(namespace))
	copy(nsKey, namespace)
	return append(nsKey, key...)
}
