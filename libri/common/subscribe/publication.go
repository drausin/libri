package subscribe

import (
	"time"
	"github.com/drausin/libri/libri/librarian/api"
	"sync"
	lru "github.com/hashicorp/golang-lru"
	"bytes"
	"github.com/drausin/libri/libri/common/id"
)


// RecentPublications tracks publications recently received from peers.
type RecentPublications interface {
	// Add tracks that a given publication was received from the given peer public key and
	// returns whether that value has recently been seen before.
	Add(pvr *pubValueReceipt) bool
}

type recentPublications struct {
	recent *lru.Cache
}

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
	pubReceipts, in := rp.recent.Get(pvr.pub.key.String())
	if !in {
		pubReceipts = newPublicationReceipts(pvr.pub.value)
	}
	pubReceipts.(*publicationReceipts).Add(pvr.receipt)
	rp.recent.Add(pvr.pub.key.String(), pubReceipts)
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

func (prs *publicationReceipts) Add(pr *pubReceipt) {
	prs.mu.Lock()
	defer prs.mu.Unlock()
	prs.receipts = append(prs.receipts, pr)
}

type keyedPub struct {
	key     id.ID
	value   *api.Publication
}

type pubReceipt struct {
	fromPub []byte
	time    time.Time
}

type pubValueReceipt struct {
	pub     *keyedPub
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
		pub: &keyedPub{
			key: valueKey,
			value: value,
		},
		receipt: &pubReceipt{
			fromPub: fromPub,
			time: time.Now().UTC(),
		},
	}, nil
}







