package storage

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/drausin/libri/libri/common/db"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
)

const (
	// MaxKeyLength is the max key length (in Bytes) for an SL.
	MaxKeyLength = 32

	// MaxValueLength is the max value length for an SL.
	MaxValueLength = 2 * 1024 * 1024 // 2 MB

	// EntriesKeyLength is the fixed length (in Bytes) of all entry keys.
	EntriesKeyLength = 32

	// MaxEntriesValueLength is the maximum length of all entry values. We add a little buffer
	// on top of the value to account for Entry values other than the actual ciphertext (which
	// we want to be <= 2MB).
	MaxEntriesValueLength = 2*1024*1024 + 1024
)

var (
	// Server namespace contains values relevant to a server.
	Server = []byte("server")

	// Client namespace contains values relevant to a client.
	Client = []byte("client")

	// Documents namespace contains all libri p2p Stored values.
	Documents = []byte("documents")
)

// NewServerSL creates a new NamespaceSL for the "server" namespace backed by a db.KVDB instance.
func NewServerSL(kvdb db.KVDB) StorerLoader {
	return NewKVDBStorerLoaderDeleter(
		Server,
		kvdb,
		NewMaxLengthChecker(MaxKeyLength),
		NewMaxLengthChecker(MaxValueLength),
	)
}

// NewClientSL creates a new NamespaceSL for the "client" namespace backed by a db.KVDB instance.
func NewClientSL(kvdb db.KVDB) StorerLoader {
	return NewKVDBStorerLoaderDeleter(
		Client,
		kvdb,
		NewMaxLengthChecker(MaxKeyLength),
		NewMaxLengthChecker(MaxValueLength),
	)
}

// DocumentStorer stores api.Document values.
type DocumentStorer interface {
	// Store an api.Document value under the given key.
	Store(key id.ID, value *api.Document) error

	Iterate(done chan struct{}, callback func(key id.ID, value []byte)) error
}

// DocumentLoader loads api.Document values.
type DocumentLoader interface {
	// Load an api.Document value with the given key.
	Load(key id.ID) (*api.Document, error)

	// Mac an api.Document value with the given key and mac key.
	Mac(key id.ID, macKey []byte) ([]byte, error)
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
	sld StorerLoaderDeleter
	c   KeyValueChecker
}

// NewDocumentSLD creates a new NamespaceSL for the "entries" namespace
// backed by a db.KVDB instance.
func NewDocumentSLD(kvdb db.KVDB) DocumentSLD {
	// TODO load metrics
	return &documentSLD{
		sld: NewKVDBStorerLoaderDeleter(
			Documents,
			kvdb,
			NewExactLengthChecker(EntriesKeyLength),
			NewMaxLengthChecker(MaxEntriesValueLength),
		),
		c: NewHashKeyValueChecker(),
	}
}

// Store checks that the key equals the SHA256 hash of the value before storing it.
func (dsld *documentSLD) Store(key id.ID, value *api.Document) error {
	if err := api.ValidateDocument(value); err != nil {
		return err
	}
	valueBytes, err := proto.Marshal(value)
	errors.MaybePanic(err) // should never happen
	keyBytes := key.Bytes()
	if err := dsld.c.Check(keyBytes, valueBytes); err != nil {
		return err
	}
	if err := dsld.sld.Store(keyBytes, valueBytes); err != nil {
		return err
	}
	return nil
}

func (dsld *documentSLD) Iterate(done chan struct{}, callback func(key id.ID, value []byte)) error {
	lb, ub := id.LowerBound.Bytes(), id.UpperBound.Bytes()
	return dsld.sld.Iterate(lb, ub, done, func(key, value []byte) {
		callback(id.FromBytes(key), value)
	})
}

func (dsld *documentSLD) Load(key id.ID) (*api.Document, error) {
	valueBytes, err := dsld.loadCheckBytes(key)
	if err != nil || valueBytes == nil {
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

func (dsld *documentSLD) Mac(key id.ID, macKey []byte) ([]byte, error) {
	valueBytes, err := dsld.loadCheckBytes(key)
	if err != nil || valueBytes == nil {
		return nil, err
	}
	macer := hmac.New(sha256.New, macKey)
	_, err = macer.Write(valueBytes)
	errors.MaybePanic(err) // should never happen b/c sha256.Write always returns nil error
	return macer.Sum(nil), nil
}

func (dsld *documentSLD) Delete(key id.ID) error {
	if err := dsld.sld.Delete(key.Bytes()); err != nil {
		return err
	}
	return nil
}

func (dsld *documentSLD) loadCheckBytes(key id.ID) ([]byte, error) {
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
	return valueBytes, nil
}
