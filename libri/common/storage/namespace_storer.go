package storage

import (
	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
)

const (
	// MaxNamespaceKeyLength is the max key length (in bytes) for a NamespaceSL.
	MaxNamespaceKeyLength = 32

	// MaxNamespaceValueLength is the max value length for a NamespaceSL.
	MaxNamespaceValueLength = 2 * 1024 * 1024 // 2 MB

	// EntriesKeyLength is the fixed length (in bytes) of all entry keys.
	EntriesKeyLength = 32

	// MaxEntriesValueLength is the maximum length of all entry values. We add a little buffer
	// on top of the value to account for Entry values other than the actual ciphertext (which
	// we want to be <= 2MB).
	MaxEntriesValueLength = 2*1024*1024 + 1024
)

var (
	// Server namespace contains values relevant to a server.
	Server Namespace = []byte("server")

	// Client namespace contains values relevant to a client.
	Client Namespace = []byte("client")

	// Documents namespace contains all libri p2p stored values.
	Documents Namespace = []byte("documents")
)

// Namespace denotes a storage namespace, which reduces to a key prefix.
type Namespace []byte

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

// NamespaceDeleter deletes a value in the configured namespace from the durable storage.
type NamespaceDeleter interface {
	Delete(key []byte) error
}

// NamespaceSL both stores and loads values in a configured namespace.
type NamespaceSL interface {
	NamespaceStorer
	NamespaceLoader
}

// NamespaceSLD stores, loads, and deletes values in a configured namespace.
type NamespaceSLD interface {
	NamespaceSL
	NamespaceDeleter
}

type namespaceSLD struct {
	ns  Namespace
	sld StorerLoaderDeleter
}

// NewServerSL creates a new NamespaceSL for the "server" namespace backed by a db.KVDB instance.
func NewServerSL(kvdb db.KVDB) NamespaceSL {
	return &namespaceSLD{
		ns: Server,
		sld: NewKVDBStorerLoaderDeleter(
			kvdb,
			NewMaxLengthChecker(MaxNamespaceKeyLength),
			NewMaxLengthChecker(MaxNamespaceValueLength),
		),
	}
}

// NewClientSL creates a new NamespaceSL for the "client" namespace backed by a db.KVDB instance.
func NewClientSL(kvdb db.KVDB) NamespaceSL {
	return &namespaceSLD{
		ns: Client,
		sld: NewKVDBStorerLoaderDeleter(
			kvdb,
			NewMaxLengthChecker(MaxNamespaceKeyLength),
			NewMaxLengthChecker(MaxNamespaceValueLength),
		),
	}
}

func (nsl *namespaceSLD) Store(key []byte, value []byte) error {
	return nsl.sld.Store(nsl.ns, key, value)
}

func (nsl *namespaceSLD) Load(key []byte) ([]byte, error) {
	return nsl.sld.Load(nsl.ns, key)
}

func (nsl *namespaceSLD) Delete(key []byte) error {
	return nsl.sld.Delete(nsl.ns, key)
}

// DocumentStorer stores api.Document values.
type DocumentStorer interface {
	// Store an api.Document value under the given key.
	Store(key id.ID, value *api.Document) error
}

// DocumentLoader loads api.Document values.
type DocumentLoader interface {
	// Load an api.Document value with the given key.
	Load(key id.ID) (*api.Document, error)
}

// DocumentDeleter deletes api.Document values.
type DocumentDeleter interface {
	// Delete the api.Document value with the given key.
	Delete(key id.ID) error
}

// DocumentSL stores & loads api.Document values.
type DocumentSL interface {
	DocumentStorer
	DocumentLoader
}

// DocumentLD stores & loads api.Document values.
type DocumentLD interface {
	DocumentLoader
	DocumentDeleter
}

// DocumentSLD stores, loads, & deletes api.Document values.
type DocumentSLD interface {
	DocumentSL
	DocumentDeleter
}

type documentSLD struct {
	sld NamespaceSLD
	c   KeyValueChecker
}

// NewDocumentSLD creates a new NamespaceSL for the "entries" namespace
// backed by a db.KVDB instance.
func NewDocumentSLD(kvdb db.KVDB) DocumentSLD {
	return &documentSLD{
		sld: &namespaceSLD{
			ns: Documents,
			sld: NewKVDBStorerLoaderDeleter(
				kvdb,
				NewExactLengthChecker(EntriesKeyLength),
				NewMaxLengthChecker(MaxEntriesValueLength),
			),
		},
		c: NewHashKeyValueChecker(),
	}
}

// Store checks that the key equals the SHA256 hash of the value before storing it.
func (dsld *documentSLD) Store(key id.ID, value *api.Document) error {
	if err := api.ValidateDocument(value); err != nil {
		return err
	}
	valueBytes, err := proto.Marshal(value)
	if err != nil {
		return err
	}
	keyBytes := key.Bytes()
	if err := dsld.c.Check(keyBytes, valueBytes); err != nil {
		return err
	}
	return dsld.sld.Store(keyBytes, valueBytes)
}

func (dsld *documentSLD) Load(key id.ID) (*api.Document, error) {
	keyBytes := key.Bytes()
	valueBytes, err := dsld.sld.Load(keyBytes)
	if err != nil {
		return nil, err
	}
	if valueBytes == nil {
		return nil, nil
	}
	if err := dsld.c.Check(keyBytes, valueBytes); err != nil {
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

func (dsld *documentSLD) Delete(key id.ID) error {
	return dsld.sld.Delete(key.Bytes())
}
