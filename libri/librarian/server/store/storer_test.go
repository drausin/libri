package store

import (
	"math/rand"
	"testing"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	ssearch "github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// TestStoreQuerier mocks the StoreQuerier interface. The Query() method returns an
// api.StoreResponse, as if the remote peer had successfully stored the value.
type TestStoreQuerier struct{
	peerID cid.ID
}

func (c *TestStoreQuerier) Query(ctx context.Context, pConn peer.Connector, rq *api.StoreRequest,
	opts ...grpc.CallOption) (*api.StoreResponse, error) {
	return &api.StoreResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: rq.Metadata.RequestId,
			PeerId: c.peerID.Bytes(),
		},
	}, nil
}

func NewTestStorer(peerID cid.ID, peersMap map[string]peer.Peer) Storer {
	return &storer{
		searcher: ssearch.NewTestSearcher(peersMap),
		q:        &TestStoreQuerier{peerID: peerID},
	}
}

func TestStorer_Store(t *testing.T) {
	n, nClosestResponses := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs := ssearch.NewTestPeers(rng, n)

	// create our searcher
	key := cid.NewPseudoRandom(rng)
	value := cid.NewPseudoRandom(rng).Bytes() // use ID for convenience, but could be anything
	storer := NewTestStorer(peers[0].ID(), peersMap)

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		search := ssearch.NewSearch(peers[0].ID(), key, &ssearch.Parameters{
			NClosestResponses: nClosestResponses,
			NMaxErrors:        DefaultNMaxErrors,
			Concurrency:       concurrency,
			Timeout:           DefaultQueryTimeout,
		})
		store := NewStore(peers[0].ID(), search, value, &Parameters{
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
