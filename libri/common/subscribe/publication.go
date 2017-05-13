package subscribe

import (
	"bytes"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	lru "github.com/hashicorp/golang-lru"
)

// KeyedPub couples a publication value and its key.
type KeyedPub struct {
	// Key is the SHA-256 has of the value's bytes.
	Key id.ID

	// Value is the publication values.
	Value *api.Publication
}

// RecentPublications tracks publications recently received from peers with an internal LRU cache.
type RecentPublications interface {

	// Get returns the *PublicationsReceipts object and an indicator of whether the object
	// exists in the cache. The *PublicationsReceipts object is nil if it doesn't exist.
	Get(envelopeKey id.ID) (*PublicationReceipts, bool)

	// Add tracks that a given publication was received from the given peer public key and
	// returns whether that value has recently been seen before.
	Add(pvr *pubValueReceipt) bool

	// Len gives the number of items in the cache.
	Len() int
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
	pubReceipts, in := rp.Get(pvr.pub.Key)
	if !in {
		pubReceipts = newPublicationReceipts(pvr.pub.Value)
	}
	pubReceipts.add(pvr.receipt)
	rp.recent.Add(pvr.pub.Key.String(), pubReceipts)
	return in
}

func (rp *recentPublications) Get(publicationKey id.ID) (*PublicationReceipts, bool) {
	pubReceipts, in := rp.recent.Get(publicationKey.String())
	if pubReceipts == nil {
		return nil, in
	}
	return pubReceipts.(*PublicationReceipts), in
}

func (rp *recentPublications) Len() int {
	return rp.recent.Len()
}

// PublicationReceipts is a list of *PubReceipts for a given publication.
type PublicationReceipts struct {
	Value    *api.Publication
	Receipts []*PubReceipt
	mu       sync.Mutex
}

func newPublicationReceipts(value *api.Publication) *PublicationReceipts {
	return &PublicationReceipts{
		Value:    value,
		Receipts: []*PubReceipt{},
	}
}

func (prs *PublicationReceipts) add(pr *PubReceipt) {
	prs.mu.Lock()
	defer prs.mu.Unlock()
	prs.Receipts = append(prs.Receipts, pr)
}

// PubReceipt represents a publication receipt from a peer (public key) at a particular time.
type PubReceipt struct {
	FromPub []byte
	Time    time.Time
}

type pubValueReceipt struct {
	pub     *KeyedPub
	receipt *PubReceipt
}

func newPublicationValueReceipt(key []byte, value *api.Publication, fromPub []byte) (
	*pubValueReceipt, error) {

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
			Key:   valueKey,
			Value: value,
		},
		receipt: &PubReceipt{
			FromPub: fromPub,
			Time:    time.Now().UTC(),
		},
	}, nil
}
