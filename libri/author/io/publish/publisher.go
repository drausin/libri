package publish

import (
	"errors"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"bytes"
)

const (
	// DefaultPutTimeout is the default timeout duration for a Publisher's Put() call to a
	// librarian.
	DefaultPutTimeout = 3 * time.Second

	// DefaultPutParallelism is the default parallelism a MultiLoadPublisher uses when
	// making multiple Put calls to librarians.
	DefaultPutParallelism = 3
)

// ErrUnexpectedMissingDocument indicates when a document is unexpectely missing from the document
// storer loader.
var ErrUnexpectedMissingDocument = errors.New("unexpected missing document")

// Publisher Puts a document into the libri network using a librarian client.
type Publisher interface {
	// Publish Puts a document using a librarian client and returns the ID of the document.
	Publish(doc *api.Document, lc api.Putter) (cid.ID, error)
}

// Parameters define configuration used by a Publisher.
type Parameters struct {
	// PutTimeout is the timeout duration used for Put requests.
	PutTimeout     time.Duration

	// PutParallelism is the number of simultaneous Put requests (for different documents) that
	// can occur.
	PutParallelism uint32
}

type publisher struct {
	clientID ecid.ID
	signer   client.Signer
	params   *Parameters
}

// NewPublisher creates a new Publisher with a given client ID, signer, and params.
func NewPublisher(clientID ecid.ID, signer client.Signer, params *Parameters) Publisher {
	return &publisher{
		clientID: clientID,
		signer:   signer,
		params:   params,
	}
}

func (p *publisher) Publish(doc *api.Document, lc api.Putter) (cid.ID, error) {
	docKey, err := api.GetKey(doc)
	if err != nil {
		return nil, err
	}
	rq := client.NewPutRequest(p.clientID, docKey, doc)
	ctx, cancel, err := client.NewSignedTimeoutContext(p.signer, rq, p.params.PutTimeout)
	if err != nil {
		return nil, err
	}
	rp, err := lc.Put(ctx, rq)
	cancel()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rq.Metadata.RequestId, rp.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}
	return docKey, nil
}

// SingleLoadPublisher publishes documents from internal storage.
type SingleLoadPublisher interface {
	// Publish loads a document with the given key and publishes them using the given
	// librarian client.
	Publish(docKey cid.ID, lc api.Putter) error
}

type singleLoadPublisher struct {
	inner Publisher
	docL storage.DocumentLoader
}

// NewSingleLoadPublisher creates a new SingleLoadPublisher from an inner Publisher and a
// storage.DocumentLoader (from which it loads the documents to publish).
func NewSingleLoadPublisher(inner Publisher, docL storage.DocumentLoader) SingleLoadPublisher {
	return &singleLoadPublisher{
		inner: inner,
		docL: docL,
	}
}

func (p *singleLoadPublisher) Publish(docKey cid.ID, lc api.Putter) error {
	pageDoc, err := p.docL.Load(docKey)
	if err != nil {
		return err
	}
	if pageDoc == nil {
		return ErrUnexpectedMissingDocument
	}
	if _, err := p.inner.Publish(pageDoc, lc); err != nil {
		return err
	}
	return nil
}

// MultiLoadPublisher loads and publishes a collection of documents from internal storage.
type MultiLoadPublisher interface {
	// Publish in parallel loads and publishes the documents with the given keys. It balances
	// between librarian clients for its Put requests.
	Publish(docKeys []cid.ID, cb api.ClientBalancer) error
}

type multiLoadPublisher struct {
	inner  SingleLoadPublisher
	params *Parameters
}

func (p *multiLoadPublisher) Publish(docKeys []cid.ID, cb api.ClientBalancer) error {
	docKeysChan := make(chan cid.ID, p.params.PutParallelism)
	go loadChan(docKeys, docKeysChan)
	wg := new(sync.WaitGroup)
	putErrs := make(chan error, 1)
	for c := uint32(0); c < p.params.PutParallelism; c++ {
		wg.Add(1)
		go func() {
			for docKey := range docKeysChan {
				if err := p.inner.Publish(docKey, cb.Next()); err != nil {
					putErrs <- err
					return
				}
			}

		}()
	}
	close(putErrs)
	close(docKeysChan)
	wg.Wait()

	select {
	case err := <-putErrs:
		return err
	default:
		return nil
	}
}

func loadChan(idSlice []cid.ID, idChan chan cid.ID) {
	for _, id := range idSlice {
		idChan <- id
	}
}
