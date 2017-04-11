package author

import (
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/author/keychain"
	"net"
	"github.com/drausin/libri/libri/librarian/api"
	"google.golang.org/grpc"
)


// sampleSelfReaderKeys samples a random pair of keys (author and reader) for the author to use
// in creating the document *Keys instance. The method returns the author and reader public keys
// along with the *Keys object.
func sampleSelfReaderKeys(
	authorKeys keychain.Keychain, selfReaderKeys keychain.Keychain,
) ([]byte, []byte, *enc.Keys, error) {

	authorID, err := authorKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	selfReaderID, err := selfReaderKeys.Sample()
	if err != nil {
		return nil, nil, nil, err
	}
	keys, err := enc.NewKeys(authorID.Key(), &selfReaderID.Key().PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return ecid.ToPublicKeyBytes(authorID), ecid.ToPublicKeyBytes(selfReaderID), keys, nil
}

func createClientBalancer(librarianAddrs []*net.TCPAddr) (api.ClientBalancer, error) {
	lcs := make([]api.LibrarianClient, len(librarianAddrs))
	for i, addr := range librarianAddrs {
		conn, err := grpc.Dial(addr.String(), grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		lcs[i] = api.NewLibrarianClient(conn)
	}
	return api.NewUniformRandomClientBalancer(lcs), nil
}