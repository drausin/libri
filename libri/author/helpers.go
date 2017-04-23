package author

import (
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/ecid"
)

type envelopeKeySampler interface {
	sample() ([]byte, []byte, *enc.Keys, error)
}

type envelopeKeySamplerImpl struct {
	authorKeys     keychain.Keychain
	selfReaderKeys keychain.Keychain
}

// sample samples a random pair of keys (author and reader) for the author to use
// in creating the document *Keys instance. The method returns the author and reader public keys
// along with the *Keys object.
func (s *envelopeKeySamplerImpl) sample() ([]byte, []byte, *enc.Keys, error) {
	authorID, err := s.authorKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	selfReaderID, err := s.selfReaderKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	keys, err := enc.NewKeys(authorID.Key(), &selfReaderID.Key().PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return ecid.ToPublicKeyBytes(authorID), ecid.ToPublicKeyBytes(selfReaderID), keys, nil
}
