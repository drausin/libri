package storage

import (
	"github.com/drausin/libri/libri/common/db"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
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

	// Documents namespace contains all libri p2p stored values.
	Documents Namespace = []byte("documents")
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

// DocumentStorer stores api.Document values.
type DocumentStorer interface {
	// Store an api.Document value under the given key.
	Store(key cid.ID, value *api.Document) error
}

// DocumentLoader loads api.Document values.
type DocumentLoader interface {
	// Load an api.Document value with the given key.
	Load(key cid.ID) (*api.Document, error)
}

type DocumentStorerLoader interface {
	DocumentStorer
	DocumentLoader
}

// keyHashNamespaceStorerLoader checks that the key equals the hash of the value before storing it.
type documentStorerLoader struct {
	nsl NamespaceStorerLoader
	c   KeyValueChecker
}

// Store checks that the key equals the SHA256 hash of the value before storing it.
func (dnsl *documentStorerLoader) Store(key cid.ID, value *api.Document) error {
	if err := api.ValidateDocument(value); err != nil {
		return err
	}
	valueBytes, err := proto.Marshal(value)
	if err != nil {
		return err
	}
	keyBytes := key.Bytes()
	if err := dnsl.c.Check(keyBytes, valueBytes); err != nil {
		return err
	}
	return dnsl.nsl.Store(keyBytes, valueBytes)
}

func (dnsl *documentStorerLoader) Load(key cid.ID) (*api.Document, error) {
	keyBytes := key.Bytes()
	valueBytes, err := dnsl.nsl.Load(keyBytes)
	if err != nil {
		return nil, err
	}
	if valueBytes == nil {
		return nil, nil
	}
	if err := dnsl.c.Check(keyBytes, valueBytes); err != nil {
		// should never happen b/c we check on Store, but being defensive just in case
		return nil, err
	}
	doc := &api.Document{}
	if err := proto.Unmarshal(valueBytes, doc); err != nil {
		return nil, err
	}
	if err := api.ValidateDocument(doc); err != nil {
		// should never happen b/c we check on Store, but being defensive just in case
		return nil, err
	}
	return doc, nil
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

// NewDocumentStorerLoader creates a new DocumentStorerLoader for the "documents" namespace.
func NewDocumentStorerLoader(sl StorerLoader) DocumentStorerLoader {
	return &documentStorerLoader{
		nsl: &namespaceStorerLoader{
			ns: Documents,
			sl: sl,
		},
		c: NewHashKeyValueChecker(),
	}
}

// NewDocumentKVDBStorerLoader creates a new NamespaceStorerLoader for the "entries" namespace
// backed by a db.KVDB instance.
func NewDocumentKVDBStorerLoader(kvdb db.KVDB) DocumentStorerLoader {
	return NewDocumentStorerLoader(
		NewKVDBStorerLoader(
			kvdb,
			NewExactLengthChecker(EntriesKeyLength),
			NewMaxLengthChecker(MaxEntriesValueLength),
		),
	)
}
