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

// StorerLoader can both store and load values.
type StorerLoader interface {
	Storer
	Loader
}

type kvdbStorerLoader struct {
	db db.KVDB
	nc Checker
	kc Checker
	vc Checker
}

// NewKVDBStorerLoader returns a new StorerLoader backed by a db.KVDB instance and with the given
// key and value checkers.
func NewKVDBStorerLoader(db db.KVDB, keyChecker Checker, valueChecker Checker) StorerLoader {
	return &kvdbStorerLoader{
		db: db,
		nc: NewMaxLengthChecker(MaxNamespaceLength),
		kc: keyChecker,
		vc: valueChecker,
	}
}

func (sl *kvdbStorerLoader) Store(namespace []byte, key []byte, value []byte) error {
	if err := sl.nc.Check(namespace); err != nil {
		return err
	}
	if err := sl.kc.Check(key); err != nil {
		return err
	}
	if err := sl.vc.Check(value); err != nil {
		return err
	}
	return sl.db.Put(namespaceKey(namespace, key), value)
}

func (sl *kvdbStorerLoader) Load(namespace []byte, key []byte) ([]byte, error) {
	if err := sl.nc.Check(namespace); err != nil {
		return nil, err
	}
	if err := sl.kc.Check(key); err != nil {
		return nil, err
	}
	return sl.db.Get(namespaceKey(namespace, key))
}

func namespaceKey(namespace []byte, key []byte) []byte {
	return append(namespace, key...)
}
