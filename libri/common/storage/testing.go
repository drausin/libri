package storage

import (
	"sync"

	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/golang/protobuf/proto"
)

// TestSLD mocks StorerLoaderDeleter interface.
type TestSLD struct {
	Bytes      []byte
	LoadErr    error
	IterateErr error
	StoreErr   error
	DeleteErr  error
	mu         sync.Mutex
}

// Load mocks StorerLoaderDeleter.Load().
func (l *TestSLD) Load(key []byte) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Bytes, l.LoadErr
}

// Iterate mocks StorerLoaderDeleter.Iterate().
func (l *TestSLD) Iterate(
	keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte),
) error {
	// "happy path" doesn't really exist ATM, implement if needed
	return l.IterateErr
}

// Store mocks StorerLoaderDeleter.Store().
func (l *TestSLD) Store(key []byte, value []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Bytes = value
	return l.StoreErr
}

// Delete mocks StorerLoaderDeleter.Delete().
func (l *TestSLD) Delete(key []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Bytes = nil
	return l.DeleteErr
}

// NewTestDocSLD creates a new TestDocSLD.
func NewTestDocSLD() *TestDocSLD {
	return &TestDocSLD{
		Stored: make(map[string]*api.Document),
	}
}

// TestDocSLD mocks DocumentSLD.
type TestDocSLD struct {
	StoreErr   error
	Stored     map[string]*api.Document
	IterateErr error
	LoadErr    error
	MacErr     error
	DeleteErr  error
	mu         sync.Mutex
}

// Store mocks DocumentSLD.Store().
func (f *TestDocSLD) Store(key id.ID, value *api.Document) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Stored[key.String()] = value
	return f.StoreErr
}

// Iterate mocks DocumentSLD.Iterate().
func (f *TestDocSLD) Iterate(done chan struct{}, callback func(key id.ID, value []byte)) error {
	for keyStr, value := range f.Stored {
		key, err := id.FromString(keyStr)
		errors.MaybePanic(err)
		valueBytes, err := proto.Marshal(value)
		errors.MaybePanic(err) // should never happen b/c only docs can be stored
		select {
		case <-done:
			return f.IterateErr
		default:
			callback(key, valueBytes)
		}
	}
	return f.IterateErr
}

// Load mocks DocumentSLD.Load().
func (f *TestDocSLD) Load(key id.ID) (*api.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value := f.Stored[key.String()]
	return value, f.LoadErr
}

// Mac mocks DocumentSLD.Mac().
func (f *TestDocSLD) Mac(key id.ID, macKey []byte) ([]byte, error) {
	return nil, f.MacErr
}

// Delete mocks DocumentSLD.Delete().
func (f *TestDocSLD) Delete(key id.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Stored, key.String())
	return f.DeleteErr
}
