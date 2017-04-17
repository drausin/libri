package ship

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/author/io/enc"
)

type Receiver interface {
	Receive(envelopeKey id.ID) (*api.Document, []id.ID, *enc.Keys, error)
}
