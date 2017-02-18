package storage

import (
	"github.com/drausin/libri/libri/db"
)

const (
	// MaxServerKeyLength is the max key length (in bytes) for the "server" namespace.
	MaxServerKeyLength = 32

	// MaxServerValueLength is the max value length for the "value" namespace.
	MaxServerValueLength = 2 * 1024 * 1024 // 2 MB

	// EntriesKeyLength is the fixed length (in bytes) of all entry keys.
	EntriesKeyLength = 32

	// MaxEntriesValueLength is the maximum length of all entry values. We add a little buffer
	// on top of the value to account for Entry values other than the actual ciphertext (which
	// we want to be <= 2MB).
	MaxEntriesValueLength = 2*1024*1024 + 1024
)

var (
	// Server namespace contains values relevant to the server itself.
	Server Namespace = []byte("server")

	// Entries namespace contains all libri p2p stored values.
	Entries Namespace = []byte("entries")
)

// Namespace denotes a storage namespace, which reduces to a key prefix.
type Namespace []byte

// Bytes returns the namespace byte encoding.
func (n Namespace) Bytes() []byte {
	return []byte(n)
}

// NamespaceStorer stores a value to durable storage under the configured namespace.
type NamespaceStorer interface {
	// Store a value for the key in the configured namespace.
	Store(key []byte, value []byte) error
}

// NamespaceLoader loads a value in the configured namespace from the durable storage.
type NamespaceLoader interface {
	// Load the value for the key in the configured namespace.
	Load(key []byte) ([]byte, error)
}

// NamespaceStorerLoader both stores and loads values in a configured namespace.
type NamespaceStorerLoader interface {
	NamespaceStorer
	NamespaceLoader
}

type namespaceStorerLoader struct {
	ns Namespace
	sl StorerLoader
}

func (nsl *namespaceStorerLoader) Store(key []byte, value []byte) error {
	return nsl.sl.Store(nsl.ns.Bytes(), key, value)
}

func (nsl *namespaceStorerLoader) Load(key []byte) ([]byte, error) {
	return nsl.sl.Load(nsl.ns.Bytes(), key)
}

// keyHashNamespaceStorerLoader checks that the key equals the hash of the value before storing it.
type keyHashNamespaceStorerLoader struct {
	nsl NamespaceStorerLoader
	c KeyValueChecker
}

// Store checks that the key equals the SHA256 hash of the value before storing it.
func (khnsl *keyHashNamespaceStorerLoader) Store(key []byte, value []byte) error {
	if err := khnsl.c.Check(key, value); err != nil {
		return err
	}
	return khnsl.nsl.Store(key, value)
}

func (khnsl *keyHashNamespaceStorerLoader) Load(key []byte) ([]byte, error) {
	value, err := khnsl.nsl.Load(key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	if err := khnsl.c.Check(key, value); err != nil {
		// should never happen b/c we check on Store, but being defensive just in case
		return nil, err
	}
	return value, nil
}

// NewServerStorerLoader creates a new NamespaceStorerLoader for the "server" namespace.
func NewServerStorerLoader(sl StorerLoader) NamespaceStorerLoader {
	return &namespaceStorerLoader{
		ns: Server,
		sl: sl,
	}
}

// NewServerKVDBStorerLoader creates a new NamespaceStorerLoader for the "server" namespace backed
// by a db.KVDB instance.
func NewServerKVDBStorerLoader(kvdb db.KVDB) NamespaceStorerLoader {
	return NewServerStorerLoader(
		NewKVDBStorerLoader(
			kvdb,
			NewMaxLengthChecker(MaxServerKeyLength),
			NewMaxLengthChecker(MaxServerValueLength),
		),
	)
}

// NewEntriesStorerLoader creates a new NamespaceStorerLoader for the "entries" namespace.
func NewEntriesStorerLoader(sl StorerLoader) NamespaceStorerLoader {
	return &keyHashNamespaceStorerLoader{
		nsl: &namespaceStorerLoader{
			ns: Entries,
			sl: sl,
		},
		c: NewHashKeyValueChecker(),
	}
}

// NewEntriesKVDBStorerLoader creates a new NamespaceStorerLoader for the "entries" namespace backed
// by a db.KVDB instance.
func NewEntriesKVDBStorerLoader(kvdb db.KVDB) NamespaceStorerLoader {
	return NewEntriesStorerLoader(
		NewKVDBStorerLoader(
			kvdb,
			NewExactLengthChecker(EntriesKeyLength),
			NewMaxLengthChecker(MaxEntriesValueLength),
		),
	)
}
