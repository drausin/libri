package publish

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	lclient "github.com/drausin/libri/libri/librarian/client"
)

const (
	// DefaultPutTimeout is the default timeout duration for a Publisher's Put() call to a
	// librarian.
	DefaultPutTimeout = 5 * time.Second

	// DefaultGetTimeout is the default timeout duration for an Acquirer's Get() call to a
	// librarian.
	DefaultGetTimeout = 5 * time.Second

	// DefaultPutParallelism is the default parallelism a MultiLoadPublisher uses when
	// making multiple Put calls to librarians.
	DefaultPutParallelism = 3

	// DefaultGetParallelism is the default parallelism a MultiStoreAcquirer uses when
	// making multiple Get calls to librarians.
	DefaultGetParallelism = 3
)

var (
	// ErrUnexpectedMissingDocument indicates when a document is unexpectedly missing from the
	// document storer loader.
	ErrUnexpectedMissingDocument = errors.New("unexpected missing document")

	// ErrPutTimeoutZeroValue indicates when the PutTimeout parameter has the zero value.
	ErrPutTimeoutZeroValue = errors.New("PutTimeout must be greater than zero")

	// ErrGetTimeoutZeroValue indicates when the GetTimeout parameter has the zero value.
	ErrGetTimeoutZeroValue = errors.New("GetTimeout must be greater than zero")

	// ErrPutParallelismZeroValue indicates when the PutParallelism parameter has the zero
	// value.
	ErrPutParallelismZeroValue = errors.New("PutParallelism must be greater than zero")

	// ErrGetParallelismZeroValue indicates when the GetParallelism parameter has the zero
	// value.
	ErrGetParallelismZeroValue = errors.New("GetParallelism must be greater than zero")

	// ErrInconsistentAuthorPubKey indicates when the document author public key is different
	// from the expected value.
	ErrInconsistentAuthorPubKey = errors.New("inconsistent author public key")
)

// Parameters define configuration used by a Publisher.
type Parameters struct {
	// PutTimeout is the timeout duration used for Put requests.
	PutTimeout time.Duration

	// GetTimeout is the timeout duration used for Get requests.
	GetTimeout time.Duration

	// PutParallelism is the number of simultaneous Put requests (for different documents) that
	// can occur.
	PutParallelism uint32

	// GetParallelism is the number of simultaneous Ge requests (for different documents) that
	// can occur.
	GetParallelism uint32
}

// NewParameters validates the parameters and returns a new *Parameters instance.
func NewParameters(
	putTimeout time.Duration,
	getTimeout time.Duration,
	putParallelism uint32,
	getParallelism uint32,
) (*Parameters, error) {

	if putTimeout == 0 {
		return nil, ErrPutTimeoutZeroValue
	}
	if getTimeout == 0 {
		return nil, ErrGetTimeoutZeroValue
	}
	if putParallelism == 0 {
		return nil, ErrPutParallelismZeroValue
	}
	if getParallelism == 0 {
		return nil, ErrGetParallelismZeroValue
	}
	return &Parameters{
		PutTimeout:     putTimeout,
		GetTimeout:     getTimeout,
		PutParallelism: putParallelism,
		GetParallelism: getParallelism,
	}, nil
}

// NewDefaultParameters creates a default *Parameters instance.
func NewDefaultParameters() *Parameters {
	params, err := NewParameters(DefaultPutTimeout, DefaultGetTimeout, DefaultPutParallelism,
		DefaultGetParallelism)
	if err != nil {
		// should never happen; if does, it's programmer error
		panic(err)
	}
	return params
}

// Publisher Puts a document into the libri network using a librarian client.
type Publisher interface {
	// Publish Puts a document using a librarian client and returns the ID of the document.
	Publish(doc *api.Document, authorPub []byte, lc api.Putter) (id.ID, error)
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

func (p *publisher) Publish(doc *api.Document, authorPub []byte, lc api.Putter) (id.ID, error) {
	docKey, err := api.GetKey(doc)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(authorPub, api.GetAuthorPub(doc)) {
		return nil, ErrInconsistentAuthorPubKey
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
	// librarian client, optionally deleting the document after it is published.
	Publish(docKey id.ID, authorPub []byte, lc api.Putter, delete bool) error
}

type singleLoadPublisher struct {
	inner Publisher
	docLD storage.DocumentLD
}

// NewSingleLoadPublisher creates a new SingleLoadPublisher from an inner Publisher and a
// storage.DocumentLD (from which it loads the documents to publish).
func NewSingleLoadPublisher(inner Publisher, docLD storage.DocumentLD) SingleLoadPublisher {
	return &singleLoadPublisher{
		inner: inner,
		docLD: docLD,
	}
}

func (p *singleLoadPublisher) Publish(
	docKey id.ID, authorPub []byte, lc api.Putter, delete bool,
) error {

	pageDoc, err := p.docLD.Load(docKey)
	if err != nil {
		return err
	}
	if pageDoc == nil {
		return ErrUnexpectedMissingDocument
	}
	if _, err := p.inner.Publish(pageDoc, authorPub, lc); err != nil {
		return err
	}
	if delete {
		return p.docLD.Delete(docKey)
	}
	return nil
}

// MultiLoadPublisher loads and publishes a collection of documents from internal storage.
type MultiLoadPublisher interface {
	// Publish in parallel loads and publishes the documents with the given keys, optionally
	// deleting them from local storage after successful delete. It balances between librarian
	// clients for its Put requests.
	Publish(docKeys []id.ID, authorPub []byte, cb api.PutterBalancer, delete bool) error
}

type multiLoadPublisher struct {
	inner  SingleLoadPublisher
	params *Parameters
}

// NewMultiLoadPublisher creates a new MultiLoadPublisher.
func NewMultiLoadPublisher(inner SingleLoadPublisher, params *Parameters) MultiLoadPublisher {
	return &multiLoadPublisher{
		inner:  inner,
		params: params,
	}
}

func (p *multiLoadPublisher) Publish(
	docKeys []id.ID, authorPub []byte, cb api.PutterBalancer, delete bool,
) error {

	rlc := lclient.NewRetryPutter(cb, p.params.PutTimeout)
	docKeysChan := make(chan id.ID, p.params.PutParallelism)
	go loadChan(docKeys, docKeysChan)
	wg := new(sync.WaitGroup)
	putErrs := make(chan error, p.params.PutParallelism)
	for c := uint32(0); c < p.params.PutParallelism; c++ {
		wg.Add(1)
		go func() {
			for docKey := range docKeysChan {
				if err := p.inner.Publish(docKey, authorPub, rlc, delete); err != nil {
					putErrs <- err
					break
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	close(putErrs)

	select {
	case err := <-putErrs:
		return err
	default:
		return nil
	}
}

func loadChan(idSlice []id.ID, idChan chan id.ID) {
	for _, id := range idSlice {
		idChan <- id
	}
	close(idChan)
}
