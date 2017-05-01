package subscribe

import (
	"time"
	"github.com/drausin/libri/libri/librarian/api"
	"sync"
	lru "github.com/hashicorp/golang-lru"
	"bytes"
	"github.com/drausin/libri/libri/common/id"
)

// KeyedPub couples a publication value and its key.
type KeyedPub struct {
	// Key is the SHA-256 has of the value's bytes.
	Key   id.ID

	// Value is the publication values.
	Value *api.Publication
}

// RecentPublications tracks publications recently received from peers with an internal LRU cache.
type RecentPublications interface {
	// Add tracks that a given publication was received from the given peer public key and
	// returns whether that value has recently been seen before.
	Add(pvr *pubValueReceipt) bool
}

type recentPublications struct {
	recent *lru.Cache
}

// NewRecentPublications creates a RecentPublications LRU cache with a given size.
func NewRecentPublications(size uint32) (RecentPublications, error) {
	// TODO (drausin) store publicationReceipts on eviction
	onEvicted := func(key interface{}, value interface{}) {}
	recent, err := lru.NewWithEvict(int(size), onEvicted)
	if err != nil {
		return nil, err
	}
	return &recentPublications{
		recent: recent,
	}, nil
}

func (rp *recentPublications) Add(pvr *pubValueReceipt) bool {
	pubReceipts, in := rp.recent.Get(pvr.pub.Key.String())
	if !in {
		pubReceipts = newPublicationReceipts(pvr.pub.Value)
	}
	pubReceipts.(*publicationReceipts).add(pvr.receipt)
	rp.recent.Add(pvr.pub.Key.String(), pubReceipts)
	return in
}

type publicationReceipts struct {
	value *api.Publication
	receipts []*pubReceipt
	mu sync.Mutex
}

func newPublicationReceipts(value *api.Publication) *publicationReceipts {
	return &publicationReceipts{
		value: value,
		receipts: []*pubReceipt{},
	}
}

func (prs *publicationReceipts) add(pr *pubReceipt) {
	prs.mu.Lock()
	defer prs.mu.Unlock()
	prs.receipts = append(prs.receipts, pr)
}


type pubReceipt struct {
	fromPub []byte
	time    time.Time
}

type pubValueReceipt struct {
	pub     *KeyedPub
	receipt *pubReceipt
}

func newPublicationValueReceipt(key []byte, value *api.Publication, fromPub []byte) (
	*pubValueReceipt, error){

	valueKey, err := api.GetKey(value)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(valueKey.Bytes(), key) {
		return nil, api.ErrUnexpectedKey
	}
	if err := api.ValidatePublicKey(fromPub); err != nil {
		return nil, err
	}
	return &pubValueReceipt{
		pub: &KeyedPub{
			Key: valueKey,
			Value: value,
		},
		receipt: &pubReceipt{
			fromPub: fromPub,
			time: time.Now().UTC(),
		},
	}, nil
}







