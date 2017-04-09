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
)

// ErrUnexpectedMissingDocument indicates when a document is unexpectely missing from the document
// storer loader.
var ErrUnexpectedMissingDocument = errors.New("unexpected missing document")

type Publisher interface {
	Publish(doc *api.Document, lc api.Putter) (cid.ID, error)
}

type Parameters struct {
	PutTimeout     time.Duration
	PutParallelism uint32
}

type publisher struct {
	clientID ecid.ID
	signer   client.Signer
	params   *Parameters
}

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
	_, err = lc.Put(ctx, rq)
	cancel()
	if err != nil {
		return nil, err
	}
	return docKey, nil
}

type SingleLoadPublisher interface {
	Publish(docKey cid.ID, lc api.LibrarianClient) error
}

type loadPublisher struct {
	inner Publisher
	docSL storage.DocumentStorerLoader
}

func (p *loadPublisher) Publish(docKey cid.ID, lc api.LibrarianClient) error {
	pageDoc, err := p.docSL.Load(docKey)
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

type MultiLoadPublisher interface {
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
