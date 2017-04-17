package publish

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/client"
	"bytes"
)

type Acquirer interface {
	// Acquire Gets a document from the libri network using a librarian client.
	Acquire(docKey id.ID, authorPub []byte, lc api.Getter) (*api.Document, error)
}

type acquirer struct {
	clientID ecid.ID
	signer   client.Signer
	params   *Parameters
}

func NewAcquirer(clientID ecid.ID, signer client.Signer, params *Parameters) Acquirer {
	return &acquirer{
		clientID: clientID,
		signer: signer,
		params: params,
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
	if !bytes.Equal(authorPub, api.GetAuthorPub(rp.Value)) {
		return nil, ErrInconsistentAuthorPubKey
	}
	return rp.Value, nil
}


type SingleStoreAcquirer interface {
	// Acquire Gets the document with the given key from the libri network and saves it to
	// internal storage.
	Acquire(docKey id.ID, authorPub []byte, lc api.Getter) error
}

type MultiStoreAcquirer interface {
	// Acquire in parellel Gets and stores the documents with the given keys. It balances
	// between librarian clients for its Put requests.
	Acquire(docKeys []id.ID, authorPub []byte, cb api.ClientBalancer) error
}