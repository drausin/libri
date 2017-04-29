package subscribe

import (
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/pkg/errors"
	"math/rand"
)

var (
	ErrOutOfBoundsFPRate = errors.New("false positive rate out of (0, 1) bounds")
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

func NewAuthorSubscription(authorPubs [][]byte, fp float64, rng *rand.Rand) (
	*api.Subscription, error) {
	return NewSubscription(authorPubs, fp, [][]byte{}, 1.0, rng)
}

func NewReaderSubscription(readerPubs [][]byte, fp float64, rng *rand.Rand) (
	*api.Subscription, error) {
	return NewSubscription([][]byte{}, 1.0, readerPubs, fp, rng)
}

func NewFPSubscription(fp float64, rng *rand.Rand) (*api.Subscription, error) {
	return NewSubscription([][]byte{}, fp, [][]byte{}, 1.0, rng)
}
