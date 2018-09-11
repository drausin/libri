package db

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	errors2 "github.com/drausin/libri/libri/common/errors"
	"github.com/tecbot/gorocksdb"
)

// KVDB is the (thin) abstraction layer of an implementation-agnostic key-value store.
type KVDB interface {
	// Get returns the value for a key.
	Get(key []byte) ([]byte, error)

	// Put stores the value for a key.
	Put(key []byte, value []byte) error

	// Delete removes the value for a key.
	Delete(key []byte) error

	// Iterate iterates through a range of key-value pairs.
	Iterate(keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte)) error

	// Close gracefully shuts down the database.
	Close()
}

// RocksDB implements the KVStore interface with a thinly wrapped RocksDB instance.
type RocksDB struct {
	// Pointer to the RocksDB object
	rdb *gorocksdb.DB

	// Read options for generic reads
	ro *gorocksdb.ReadOptions

	// Write options for generic writes
	wo *gorocksdb.WriteOptions

	// ensures only one iteration of DB at a time
	iterMu *sync.Mutex
}

// NewRocksDB creates a new RocksDB instance with default read and write options.
func NewRocksDB(dbDir string) (*RocksDB, error) {
	err := os.MkdirAll(dbDir, os.ModePerm)
	if err != nil {
		return nil, err
	}
	options := newRocksDBOptimizedOptions()
	db, err := gorocksdb.OpenDb(options, dbDir)
	if err != nil {
		return nil, err
	}

	return &RocksDB{
		rdb:    db,
		ro:     gorocksdb.NewDefaultReadOptions(),
		wo:     gorocksdb.NewDefaultWriteOptions(),
		iterMu: new(sync.Mutex),
	}, nil
}

func newRocksDBDefaultOptions() *gorocksdb.Options {
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	return opts
}

func newRocksDBOptimizedOptions() *gorocksdb.Options {
	// TODO (drausin) figure out best way to parameterize this
	opts := newRocksDBDefaultOptions()
	opts.IncreaseParallelism(4)

	bbtOpts := gorocksdb.NewDefaultBlockBasedTableOptions()
	bbtOpts.SetBlockCache(gorocksdb.NewLRUCache(1024 * 1024 * 1024)) // 1 GB
	bbtOpts.SetBlockSize(32 * 1024)                                  // 32K
	bbtOpts.SetFilterPolicy(gorocksdb.NewBloomFilter(10))
	opts.SetBlockBasedTableFactory(bbtOpts)

	opts.SetAllowConcurrentMemtableWrites(true)
	opts.OptimizeLevelStyleCompaction(500 * 1024 * 1024) // 500 MB memtable
	opts.SetStatsDumpPeriodSec(10 * 60)
	return opts
}

// NewTempDirRocksDB creates a new RocksDB instance (used mostly for local testing) in a local
// temporary directory.
func NewTempDirRocksDB() (*RocksDB, func(), error) {
	dir, err := ioutil.TempDir("", "kvdb-test-rocksdb")
	cleanup := func() {
		rmErr := os.RemoveAll(dir)
		if rmErr != nil {
			panic(rmErr)
		}
	}
	if err != nil {
		return nil, cleanup, err
	}
	rdb, err := NewRocksDB(dir)
	return rdb, cleanup, err
}

// Get returns the value for a key.
func (db *RocksDB) Get(key []byte) ([]byte, error) {
	// Return copy of bytes instead of a slice to make it simpler for the user. If this proves
	// slow for large reads we might want to add a separate method for getting the slice
	// (or an abstraction of it) directly.
	if db.rdb == nil {
		return nil, errors.New("rdb is nil")
	}
	return db.rdb.GetBytes(db.ro, key)
}

// Put stores the value for a key.
func (db *RocksDB) Put(key []byte, value []byte) error {
	return db.rdb.Put(db.wo, key, value)
}

// Delete removes the value for a key.
func (db *RocksDB) Delete(key []byte) error {
	return db.rdb.Delete(db.wo, key)
}

// Iterate iterates over the values in the DB.
func (db *RocksDB) Iterate(
	keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte),
) error {
	db.iterMu.Lock()
	defer db.iterMu.Unlock()
	opts := gorocksdb.NewDefaultReadOptions()
	opts.SetIterateUpperBound(keyUB)
	opts.SetFillCache(false)
	iter := db.rdb.NewIterator(opts)
	defer iter.Close()
	defer opts.Destroy()

	iter.Seek(keyLB)
	for ; iter.Valid(); iter.Next() {
		callback(iter.Key().Data(), iter.Value().Data())
		select {
		case <-done:
			return iter.Err()
		default:
			// continue
		}
	}
	return iter.Err()
}

// Close gracefully shuts down the database.
func (db *RocksDB) Close() {
	db.rdb.Close()
}

// NewMemoryDB creates a new in-memory KVDB.
func NewMemoryDB() *Memory {
	return &Memory{data: make(map[string][]byte)}
}

// Memory contains an in-memory KVDB.
type Memory struct {
	data map[string][]byte
	mu   sync.Mutex
}

// Get returns the value for a key.
func (db *Memory) Get(key []byte) ([]byte, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	value := db.data[getDataKey(key)]
	return value, nil
}

// Put stores the value for a key.
func (db *Memory) Put(key []byte, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[getDataKey(key)] = value
	return nil
}

// Delete removes the value for a key.
func (db *Memory) Delete(key []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, getDataKey(key))
	return nil
}

// Iterate iterates over the values in the DB.
func (db *Memory) Iterate(
	keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte),
) error {
	dataKeyLB, dataKeyUB := getDataKey(keyLB), getDataKey(keyUB)
	db.mu.Lock()
	defer db.mu.Unlock()
	for key, value := range db.data {
		if strings.Compare(key, dataKeyLB) >= 0 && strings.Compare(key, dataKeyUB) < 0 {
			keyBytes, err := hex.DecodeString(key)
			errors2.MaybePanic(err)
			callback(keyBytes, value)
			select {
			case <-done:
				return nil
			default:
				// continue
			}
		}
	}
	return nil
}

// Close gracefully shuts down the database.
func (db *Memory) Close() {}

func getDataKey(key []byte) string {
	return fmt.Sprintf("%x", key)
}
