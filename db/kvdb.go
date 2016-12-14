package db

import (
	"github.com/tecbot/gorocksdb"
)

// KVDB is the (thin) abstraction layer of an implementation-agnostic key-value store.
type KVDB interface {

	Get(key []byte) ([]byte, error)

	Put(key []byte, value []byte) error

	Delete(key []byte) error
}

// RocksDB implements the KVStore interface via RocksDB implementation.
type RocksDB struct {

	// Pointer to the RocksDB object
	rdb *gorocksdb.DB

	// Read options for generic reads
	ro *gorocksdb.ReadOptions

	// Write options for generic writes
	wo *gorocksdb.WriteOptions
}

// NewRocksDB creates a new RocksDB instance with default read and write options.
func NewRocksDB(dbDir string) (*RocksDB, error) {
	options := gorocksdb.NewDefaultOptions()
	options.SetCreateIfMissing(true)
	db, err := gorocksdb.OpenDb(options, dbDir)
	if err != nil  {
		return nil, err
	}

	return &RocksDB{
		rdb: db,
		ro: gorocksdb.NewDefaultReadOptions(),
		wo: gorocksdb.NewDefaultWriteOptions(),
	}, nil
}

// Get returns a copy of the value for a given key.
func (db *RocksDB) Get(key []byte) ([]byte, error) {
	// Return copy of bytes instead of a slice to make it simpler for the user. If this proves slow for large reads
	// we might want to add a separate method for getting the slice (or an abstraction of it) directly.
	return db.rdb.GetBytes(db.ro, key)
}

// Put sets the value for a given key.
func (db *RocksDB) Put(key []byte, value []byte) error {
	return db.rdb.Put(db.wo, key, value)
}

// Delete removes the value of the given key.
func (db *RocksDB) Delete(key []byte) error {
	return db.rdb.Delete(db.wo, key)
}