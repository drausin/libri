package subscribe

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"math/rand"
)

var (
	// ErrOutOfBoundsFPRate indicates when the false positive rate is not in (0, 1].
	ErrOutOfBoundsFPRate = errors.New("false positive rate out of (0, 1] bounds")

	// ErrNilPublicKeys indicates when either the author or reader public keys are nil.
	ErrNilPublicKeys = errors.New("nil public keys")
)

// NewSubscription returns a new subscription with filters for the given author and reader public
// keys and false positive rates. Users should recall that the overall subscription false positive
// rate will be the product of the author and reader false positive rates.
func NewSubscription(
	authorPubs [][]byte,
	authorFp float64,
	readerPubs [][]byte,
	readerFp float64,
	rng *rand.Rand,
) (*api.Subscription, error) {

	if readerFp <= 0.0 || readerFp > 1.0 || authorFp <= 0.0 || authorFp > 1.0 {
		return nil, ErrOutOfBoundsFPRate
	}
	if authorPubs == nil || readerPubs == nil {
		return nil, ErrNilPublicKeys
	}
	authorFilter, err := ToAPI(newFilter(authorPubs, authorFp, rng))
	if err != nil {
		return nil, err
	}
	readerFilter, err := ToAPI(newFilter(readerPubs, readerFp, rng))
	if err != nil {
		return nil, err
	}
	return &api.Subscription{
		AuthorPublicKeys: authorFilter,
		ReaderPublicKeys: readerFilter,
	}, nil
}

// NewAuthorSubscription creates an *api.Subscription using the given author public keys and false
// positive rate with a 1.0 false positive rate for reader keys. This is useful when one doesn't
// want to filter on any reader keys.
func NewAuthorSubscription(authorPubs [][]byte, fp float64, rng *rand.Rand) (
	*api.Subscription, error) {
	return NewSubscription(authorPubs, fp, [][]byte{}, 1.0, rng)
}

// NewReaderSubscription creates an *api.Subscription using the given reader public keys and false
// positive rate with a 1.0 false positive rate for author keys. This is useful when one doesn't
// want to filter on any author keys.
func NewReaderSubscription(readerPubs [][]byte, fp float64, rng *rand.Rand) (
	*api.Subscription, error) {
	return NewSubscription([][]byte{}, 1.0, readerPubs, fp, rng)
}

// NewFPSubscription creates an *api.Subscription with the given false positive rate on the author
// keys and a 1.0 false positive rate on the reader keys.
func NewFPSubscription(fp float64, rng *rand.Rand) (*api.Subscription, error) {
	return NewSubscription([][]byte{}, fp, [][]byte{}, 1.0, rng)
}
