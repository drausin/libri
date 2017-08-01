package replicate

import (
	"github.com/drausin/libri/libri/common/id"
	"time"
	"github.com/drausin/libri/libri/librarian/server/store"
)


type ReplicateResult struct {
	verify *VerifyResult
	store  *store.Result
}

type Replicate struct {
	Key id.ID
	MAC []byte
}
