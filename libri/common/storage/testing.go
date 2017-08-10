package storage

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"sync"
)

type TestSLD struct {
	Bytes      []byte
	LoadErr    error
	IterateErr error
	StoreErr   error
	DeleteErr error
}

func (l *TestSLD) Load(key []byte) ([]byte, error) {
	return l.Bytes, l.LoadErr
}

func (l *TestSLD) Iterate(
	keyLB, keyUB []byte, done chan struct{}, callback func(key, value []byte),
) error {
	return l.IterateErr
}

func (l *TestSLD) Store(key []byte, value []byte) error {
	l.Bytes = value
	return l.StoreErr
}

func (l *TestSLD) Delete(key []byte) error {
	return l.DeleteErr
}

func NewTestDocSLD() *TestDocSLD {
	return &TestDocSLD{
		Stored: make(map[string]*api.Document),
	}
}

type TestDocSLD struct {
	StoreErr   error
	Stored     map[string]*api.Document
	IterateErr error
	LoadErr    error
	MacErr     error
	DeleteErr  error
	mu         sync.Mutex
}

func (f *TestDocSLD) Store(key id.ID, value *api.Document) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Stored[key.String()] = value
	return f.StoreErr
}

func (f *TestDocSLD) Iterate(done chan struct{}, callback func(key id.ID, value []byte)) error {
	return f.IterateErr
}

func (f *TestDocSLD) Load(key id.ID) (*api.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value := f.Stored[key.String()]
	return value, f.LoadErr
}

func (f *TestDocSLD) Mac(key id.ID, macKey []byte) ([]byte, error) {
	return nil, f.MacErr
}

func (f *TestDocSLD) Delete(key id.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Stored, key.String())
	return f.DeleteErr
}
