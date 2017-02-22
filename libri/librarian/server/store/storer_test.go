package store

import (
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/gogo/protobuf/proto"
)

func TestNewDefaultStorer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	s := NewDefaultStorer(ecid.NewPseudoRandom(rng))
	assert.NotNil(t, s.(*storer).signer)
	assert.NotNil(t, s.(*storer).searcher)
	assert.NotNil(t, s.(*storer).querier)
}

// TestStoreQuerier mocks the StoreQuerier interface. The Query() method returns an
// api.StoreResponse, as if the remote peer had successfully stored the value.
type TestStoreQuerier struct {
	peerID ecid.ID
}

func (c *TestStoreQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	return &api.StoreResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: rq.Metadata.RequestId,
			PubKey:    ecid.ToPublicKeyBytes(c.peerID),
		},
	}, nil
}

// NoOpSigner implements the signature.Signer interface but just returns a dummy token.
type NoOpSigner struct{}

// Sign returns a dummy token.
func (s *NoOpSigner) Sign(m proto.Message) (string, error) {
	return "noop.token.sig", nil
}

func NewTestStorer(peerID ecid.ID, peersMap map[string]peer.Peer) Storer {
	return &storer{
		searcher: ssearch.NewTestSearcher(peersMap),
		querier:  &TestStoreQuerier{peerID: peerID},
		signer: &NoOpSigner{},
	}
}

func TestStorer_Store_ok(t *testing.T) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything
	storer := NewTestStorer(selfID, peersMap)

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		search := ssearch.NewSearch(selfID, key, &ssearch.Parameters{
			NClosestResponses: nClosestResponses,
			NMaxErrors:        DefaultNMaxErrors,
			Concurrency:       concurrency,
			Timeout:           DefaultQueryTimeout,
		})
		store := NewStore(selfID, search, value, &Parameters{
			NMaxErrors:  DefaultNMaxErrors,
			Concurrency: concurrency,
		})

		// init the seeds of our search: usually this comes from the routing.Table.Peak()
		// method, but we'll just allocate directly
		seeds := make([]peer.Peer, len(selfPeerIdxs))
		for i := 0; i < len(selfPeerIdxs); i++ {
			seeds[i] = peers[selfPeerIdxs[i]]
		}

		// do the search!
		err := storer.Store(store, seeds)

		// checks
		assert.Nil(t, err)
		assert.True(t, store.Finished())
		assert.True(t, store.Stored())
		assert.False(t, store.Errored())

		assert.Equal(t, nClosestResponses, uint(len(store.Result.Responded)))
		assert.Equal(t, 0, len(store.Result.Unqueried))
		assert.Equal(t, uint(0), store.Result.NErrors)
		assert.Nil(t, store.Result.FatalErr)
	}
}

type errConnector struct{}

func (ec *errConnector) Connect() (api.LibrarianClient, error) {
	return nil, errors.New("some connect error")
}

func (ec *errConnector) Disconnect() error {
	return nil
}

func TestStorer_Store_err_connector(t *testing.T) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := ssearch.NewTestPeers(rng, n)
	for i, p := range peers {
		// replace peer with connector that errors
		peers[i] = peer.New(p.ID(), "", &errConnector{})
	}

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything
	storer := NewTestStorer(selfID, peersMap)

	concurrency := uint(1)
	search := ssearch.NewSearch(selfID, key, &ssearch.Parameters{
		NClosestResponses: nClosestResponses,
		NMaxErrors:        DefaultNMaxErrors,
		Concurrency:       concurrency,
		Timeout:           DefaultQueryTimeout,
	})
	store := NewStore(selfID, search, value, &Parameters{
		NMaxErrors:  DefaultNMaxErrors,
		Concurrency: concurrency,
	})

	// init the seeds of our search: usually this comes from the routing.Table.Peak()
	// method, but we'll just allocate directly
	seeds := make([]peer.Peer, len(selfPeerIdxs))
	for i := 0; i < len(selfPeerIdxs); i++ {
		seeds[i] = peers[selfPeerIdxs[i]]
	}

	// do the search!
	err := storer.Store(store, seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, store.Exhausted())  // since we can't connect to any of the peers
	assert.False(t, store.Stored())
	assert.False(t, store.Errored())

	assert.Equal(t, 0, len(store.Result.Responded))
	assert.Equal(t, 0, len(store.Result.Unqueried))
	assert.Equal(t, uint(0), store.Result.NErrors)
	assert.Nil(t, store.Result.FatalErr)
}

type errSearcher struct{}

func (es *errSearcher) Search(search *ssearch.Search, seeds []peer.Peer) error {
	return errors.New("some search error")
}

func TestStorer_Store_err(t *testing.T) {
	s := &storer{
		searcher: &errSearcher{},
	}

	// check that Store() surfaces searcher error
	store := &Store{
		Result: &Result{},
	}
	assert.NotNil(t, s.Store(store, nil))
}

func TestQuerier_Query_err(t *testing.T) {
	c := &errConnector{}
	q := NewQuerier()

	// check that error from c.Connect() surfaces to q.Query(...)
	_, err := q.Query(nil, c, nil, nil)
	assert.NotNil(t, err)
}

// Explanation: ideally would have unit test like this, but mocking an api.LibrarianClient is
// annoying b/c there are so many service methods. Will have to rely on integration tests to cover
// this branch.
//
// func TestQuerier_Query_ok(t *testing.T) {}
