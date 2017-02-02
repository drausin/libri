package storage

import (
	"github.com/drausin/libri/libri/db"
	"github.com/pkg/errors"
	"fmt"
	cid "github.com/drausin/libri/libri/common/id"
)

const (
	MaxValueLength = 2 * 1024 * 1024 // 2MB
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

// StoreLoader can both store and load values.
type StorerLoader interface {
	Storer
	Loader
}

type kvdbStorerLoader struct {
	db db.KVDB
	kc KeyChecker
	vc ValueChecker
}

// NewKVDBStorerLoader returns a new StorerLoader backed by a db.KVDB instance.
func NewKVDBStorerLoader(db db.KVDB) StorerLoader {
	return &kvdbStorerLoader{
		db: db,
		kc: NewKeyChecker(),
		vc: NewValueChecker(),
	}
}

func (sl *kvdbStorerLoader) Store(namespace []byte, key []byte, value []byte) error {
	if err := sl.kc.Check(key); err != nil {
		return err
	}
	if err := sl.vc.Check(value); err != nil {
		return err
	}
	return sl.db.Put(namespaceKey(namespace, key), value)
}

func (sl *kvdbStorerLoader) Load(namespace []byte, key []byte) ([]byte, error) {
	if err := sl.kc.Check(key); err != nil {
		return nil, err
	}
	return sl.db.Get(namespaceKey(namespace, key))
}

func namespaceKey(namespace []byte, key []byte) []byte {
	return append(namespace, key...)
}

// KeyChecker checks that a key is valid.
type KeyChecker interface {
	// Check that key is valid.
	Check(key []byte) error
}

type keySizeChecker struct {}

func NewKeyChecker() KeyChecker {
	return &keySizeChecker{}
}

func (kc *keySizeChecker) Check(key []byte) error {
	if key == nil {
		return errors.New("key must not be nil")
	}
	if len(key) != cid.Length {
		return fmt.Errorf("key (length = %v) must have length %v", len(key), cid.Length)
	}
	return nil
}

// ValueChecker checks that a value is valid.
type ValueChecker interface {
	// Check that value is valid.
	Check(value []byte) error
}

type valueSizeChecker struct {}

func NewValueChecker() ValueChecker {
	return &valueSizeChecker{}
}

func (kc *valueSizeChecker) Check(value []byte) error {
	if value == nil {
		return errors.New("unwilling to store nil values")
	}
	if len(value) == 0 {
		return errors.New("unwilling to store values with length 0")
	}
	if len(value) > MaxValueLength {
		return fmt.Errorf("unwilling to store values with length %v > %v", len(value),
			MaxValueLength)
	}
	return nil
}
