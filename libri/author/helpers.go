package author

import (
	"net"

	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/author/keychain"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type envelopeKeySampler interface {
	sample() ([]byte, []byte, *enc.KEK, *enc.EEK, error)
}

type envelopeKeySamplerImpl struct {
	authorKeys     keychain.GetterSampler
	selfReaderKeys keychain.GetterSampler
}

// sample samples a random pair of keys (author and reader) for the author to use
// in creating the document *Keys instance. The method returns the author and reader public keys
// along with the *Keys object.
func (s *envelopeKeySamplerImpl) sample() ([]byte, []byte, *enc.KEK, *enc.EEK, error) {
	authorID, err := s.authorKeys.Sample()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	selfReaderID, err := s.selfReaderKeys.Sample()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kek, err := enc.NewKEK(authorID.Key(), &selfReaderID.Key().PublicKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	eek, err := enc.NewEEK()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return authorID.PublicKeyBytes(), selfReaderID.PublicKeyBytes(), kek, eek, nil
}

// use var so it's easy to replace for tests w/o a single-method interface
var getLibrarianHealthClients = func(
	librarianAddrs []*net.TCPAddr,
) (map[string]healthpb.HealthClient, error) {

	healthClients := make(map[string]healthpb.HealthClient)
	for _, librarianAddr := range librarianAddrs {
		addrStr := librarianAddr.String()
		conn, err := grpc.Dial(addrStr, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		healthClients[addrStr] = healthpb.NewHealthClient(conn)
	}
	return healthClients, nil
}
