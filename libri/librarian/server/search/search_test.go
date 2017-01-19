package search

import (
	"testing"
	"math/rand"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"container/heap"
	"github.com/stretchr/testify/assert"
	"net"
	"fmt"
	"github.com/drausin/libri/libri/librarian/api"
	"google.golang.org/grpc"
	"golang.org/x/net/context"
)

func TestClosePeers_MinHeap(t *testing.T) {
	testClosePeers(t, true)
}

func TestClosePeers_MaxHeap(t *testing.T) {
	testClosePeers(t, false)
}

func testClosePeers(t *testing.T, minHeap bool) {
	rng := rand.New(rand.NewSource(int64(0)))
	target := id.NewPseudoRandom(rng)
	var cp *closePeers
	if minHeap {
		cp = newClosePeersMinHeap(target)
	} else {
		cp = newClosePeersMaxHeap(target)
	}

	// add the peers
	for _, p := range peer.NewTestPeers(rng, 20) {
		heap.Push(cp, p)
	}

	// make sure our peak methods behave as expected
	curDist := cp.PeakDistance()
	assert.Equal(t, curDist, target.Distance(cp.PeakPeer().ID()))
	assert.Equal(t, curDist, cp.distance(cp.PeakPeer()))
	heap.Pop(cp)

	for ; cp.Len() > 0; heap.Pop(cp) {
		if minHeap {
			// each subsequent peer should be farther away
			assert.True(t, curDist.Cmp(cp.PeakDistance()) < 0)
		} else {
			// each subsequent peer should be closer
			assert.True(t, curDist.Cmp(cp.PeakDistance()) > 0)
		}
		curDist = cp.PeakDistance()
	}
}


type clientMock struct {
	addresses []*api.PeerAddress
}

func (c *clientMock) Ping(ctx context.Context, in *api.PingRequest, opts ...grpc.CallOption) (
	*api.PingResponse, error) {
	return nil, nil
}

func (c *clientMock) Identify(ctx context.Context, in *api.IdentityRequest,
	opts ...grpc.CallOption) (*api.IdentityResponse, error) {
	return nil, nil
}

func (c *clientMock) FindPeers(ctx context.Context, in *api.FindRequest,
	opts ...grpc.CallOption) (*api.FindPeersResponse, error) {
	return &api.FindPeersResponse{RequestId: in.RequestId, Addresses: c.addresses}, nil
}


func TestFind(t *testing.T) {
	// create n peers
	// randomly add k < n peers to each peer
	// for C times
	// - create target
	// - Find()
	// - create set of found peers
	// - assert all other peers are farther away
	n := 32
	peers := make([]peer.Peer, n)
	rng := rand.New(rand.NewSource(int64(0)))
	for i := 0; i < n; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", 11000+i))
		if err != nil {
			panic(err)
		}
		peers[i] = peer.New(id.NewPseudoRandom(rng), fmt.Sprintf("peer-%02d", i),
			peer.NewConnector(addr))
	}


}
