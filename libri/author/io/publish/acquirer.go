package publish

import (
	"bytes"
	"sync"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	lclient "github.com/drausin/libri/libri/librarian/client"
)

// Acquirer Gets documents from the libri network.
type Acquirer interface {
	// Acquire Gets a document from the libri network using a librarian client.
	Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (*api.Document, error)
}

type acquirer struct {
	clientID ecid.ID
	signer   client.Signer
	params   *Parameters
}

// NewAcquirer creates a new Acquirer with the given clientID signer, and params.
func NewAcquirer(clientID ecid.ID, signer client.Signer, params *Parameters) Acquirer {
	return &acquirer{
		clientID: clientID,
		signer:   signer,
		params:   params,
	}
}

func (a *acquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (*api.Document, error) {
	rq := client.NewGetRequest(a.clientID, docKey)
	ctx, cancel, err := client.NewSignedTimeoutContext(a.signer, rq, a.params.GetTimeout)
	if err != nil {
		return nil, err
	}
	rp, err := lc.Get(ctx, rq)
	cancel()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rq.Metadata.RequestId, rp.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}
	if err := api.ValidateDocument(rp.Value); err != nil {
		return nil, err
	}
	return rp.Value, nil
}

// SingleStoreAcquirer Gets a document and saves it to internal storage.
type SingleStoreAcquirer interface {
	// Acquire Gets the document with the given key from the libri network and saves it to
	// internal storage.
	Acquire(docKey id.ID, authorPub []byte, lc api.Getter) error
}

type singleStoreAcquirer struct {
	inner Acquirer
	docS  storage.DocumentStorer
}

// NewSingleStoreAcquirer creates a new SingleStoreAcquirer from the inner Acquirer and the docS
// document storer.
func NewSingleStoreAcquirer(inner Acquirer, docS storage.DocumentStorer) SingleStoreAcquirer {
	return &singleStoreAcquirer{
		inner: inner,
		docS:  docS,
	}
}

func (a *singleStoreAcquirer) Acquire(docKey id.ID, authorPub []byte, lc api.Getter) error {
	doc, err := a.inner.Acquire(docKey, authorPub, lc)
	if err != nil {
		return err
	}
	if err := a.docS.Store(docKey, doc); err != nil {
		return err
	}
	return nil
}

// MultiStoreAcquirer Gets and stores multiple documents.
type MultiStoreAcquirer interface {
	// Acquire in parallel Gets and stores the documents with the given keys. It balances
	// between librarian clients for its Put requests.
	Acquire(docKeys []id.ID, authorPub []byte, cb client.GetterBalancer) error

	// GetRetryGetter returns a new retrying api.Getter.
	GetRetryGetter(cb client.GetterBalancer) api.Getter
}

type multiStoreAcquirer struct {
	inner  SingleStoreAcquirer
	params *Parameters
}

// NewMultiStoreAcquirer creates a new MultiStoreAcquirer from the inner SingleStoreAcquirer and
// params.
func NewMultiStoreAcquirer(inner SingleStoreAcquirer, params *Parameters) MultiStoreAcquirer {
	return &multiStoreAcquirer{
		inner:  inner,
		params: params,
	}
}

func (a *multiStoreAcquirer) Acquire(
	docKeys []id.ID, authorPub []byte, cb client.GetterBalancer,
) error {

	rlc := a.GetRetryGetter(cb)
	docKeysChan := make(chan id.ID, a.params.PutParallelism)
	go loadChan(docKeys, docKeysChan)
	wg := new(sync.WaitGroup)
	getErrs := make(chan error, a.params.GetParallelism)
	for c := uint32(0); c < a.params.GetParallelism; c++ {
		wg.Add(1)
		go func() {
			for docKey := range docKeysChan {
				if err := a.inner.Acquire(docKey, authorPub, rlc); err != nil {
					getErrs <- err
					break
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(getErrs)

	select {
	case err := <-getErrs:
		return err
	default:
		return nil
	}
}

func (p *multiStoreAcquirer) GetRetryGetter(cb client.GetterBalancer) api.Getter {
	return lclient.NewRetryGetter(cb, true, p.params.GetTimeout)
}
