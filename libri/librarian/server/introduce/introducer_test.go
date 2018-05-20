package introduce

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"sync"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	lclient "github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestNewDefaultIntroducer(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	p, err := lclient.NewDefaultLRUPool()
	assert.Nil(t, err)
	s := NewDefaultIntroducer(
		lclient.NewSigner(ecid.NewPseudoRandom(rng).Key()),
		&fixedRecorder{},
		id.NewPseudoRandom(rng),
		p,
	)
	assert.NotNil(t, s.(*introducer).signer)
	assert.NotNil(t, s.(*introducer).introducerCreator)
	assert.NotNil(t, s.(*introducer).repProcessor)
	assert.NotNil(t, s.(*introducer).rec)
}

func TestIntroducer_Introduce_ok(t *testing.T) {
	for concurrency := uint(1); concurrency <= 3; concurrency++ {
		rec := &fixedRecorder{}
		introducer, intro, selfPeerIdxs, peers := newTestIntros(concurrency, rec)
		seeds := search.NewTestSeeds(peers, selfPeerIdxs[:3])

		// do the intro!
		err := introducer.Introduce(intro, seeds)

		// checks
		assert.Nil(t, err)
		assert.True(t, intro.Finished())
		assert.True(t, intro.ReachedTarget())
		assert.False(t, intro.Exhausted())
		assert.False(t, intro.Errored())
		assert.Equal(t, uint(0), intro.Result.NErrors)
		assert.True(t, len(intro.Result.Responded) >=
			int(intro.Params.TargetNumIntroductions))

		// make sure at least one of the seeds has been removed from the unqueried map
		nUnqueriedSeeds := 0
		for i := range seeds {
			if _, in := intro.Result.Unqueried[fmt.Sprintf("seed%02d", i)]; in {
				nUnqueriedSeeds++
			}
		}
		assert.True(t, len(seeds) > nUnqueriedSeeds)

		// check successes recorded properly
		assert.True(t, len(intro.Result.Responded) <= rec.nSuccesses)
		assert.Zero(t, rec.nErrors)
	}
}

func TestIntroducer_Introduce_queryErr(t *testing.T) {
	rec := &fixedRecorder{}
	introducerImpl, intro, selfPeerIdxs, peers := newTestIntros(1, rec)
	seeds := search.NewTestSeeds(peers, selfPeerIdxs)

	// all queries return errors as if they'd timed out
	introducerImpl.(*introducer).introducerCreator = &fixedIntroducerCreator{
		err: errors.New("some Create error"),
	}

	// do the intro!
	err := introducerImpl.Introduce(intro, seeds)

	// checks
	assert.Nil(t, err)
	assert.True(t, intro.Finished())
	assert.True(t, intro.Errored()) // since all queries returned errors
	assert.False(t, intro.Exhausted())
	assert.False(t, intro.ReachedTarget())
	assert.Equal(t, intro.Params.NMaxErrors, intro.Result.NErrors)
	assert.Nil(t, intro.Result.FatalErr)
	assert.Equal(t, 0, len(intro.Result.Responded))

	// check successes recorded properly
	assert.Zero(t, rec.nSuccesses)
	assert.Equal(t, int(intro.Result.NErrors), rec.nErrors)
}

func TestIntroducer_query_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	intro, introducers := newQueryTestIntroduction()
	introducerImpl := &introducer{
		signer: &lclient.TestNoOpSigner{},
		introducerCreator: &fixedIntroducerCreator{
			introducers: introducers,
		},
	}

	next := peer.NewTestPeer(rng, 0)
	rp, err := introducerImpl.query(next, intro)

	assert.Nil(t, err)
	assert.NotNil(t, rp.Metadata.RequestId)
}

func TestIntroducer_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	intro, _ := newQueryTestIntroduction()
	next := peer.NewTestPeer(rng, 0)
	cases := []*introducer{
		// case 0
		{
			signer:            &lclient.TestNoOpSigner{},
			introducerCreator: &fixedIntroducerCreator{err: errors.New("some Create error")},
		},

		// case 1
		{
			signer:            &lclient.TestErrSigner{},
			introducerCreator: &fixedIntroducerCreator{},
		},

		// case 2
		{
			signer: &lclient.TestNoOpSigner{},
			introducerCreator: &fixedIntroducerCreator{
				err: errors.New("some Store error"),
			},
		},

		// case 3
		{
			signer: &lclient.TestNoOpSigner{},
			introducerCreator: &fixedIntroducerCreator{
				introducers: map[string]api.Introducer{
					next.Address().String(): &fixedIntroducer{requestID: []byte{1, 2, 3, 4}},
				},
			},
		},
	}
	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp1, err := c.query(next, intro)
		assert.Nil(t, rp1, info)
		assert.NotNil(t, err, info)
	}
}

func TestResponseProcessor_Process(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	nPeers := 16
	responder := peer.NewTestPeer(rng, nPeers)
	peers := peer.NewTestPeers(rng, nPeers)
	selfPeer := peer.NewTestPeer(rng, nPeers+1)
	rp := NewResponseProcessor(peer.NewFromer(), selfPeer.ID())
	apiPeers := peer.ToAPIs(peers)
	apiPeers = append(apiPeers, selfPeer.ToAPI())

	result := NewInitialResult()

	response1 := &api.IntroduceResponse{
		Self:  responder.ToAPI(),
		Peers: apiPeers,
	}
	rp.Process(response1, result)

	// make sure we've added the responder as responded
	_, in := result.Responded[responder.ID().String()]
	assert.True(t, in)

	// make sure we've added each peer to the unqueried map
	assert.Equal(t, nPeers, len(result.Unqueried))
	for _, p := range peers {
		_, in = result.Unqueried[p.ID().String()]
		assert.True(t, in)
	}

	// make sure selfPeer isn't in the unqueried map
	_, in = result.Unqueried[selfPeer.ID().String()]
	assert.False(t, in)

	// make in identical response
	response2 := &api.IntroduceResponse{
		Self:  responder.ToAPI(),
		Peers: peer.ToAPIs(peers),
	}
	rp.Process(response2, result)

	// make sure nothing has changed with Unqueried map
	assert.Equal(t, nPeers, len(result.Unqueried))
	for _, p := range peers {
		_, in = result.Unqueried[p.ID().String()]
		assert.True(t, in)
	}
}

func newQueryTestIntroduction() (*Introduction, map[string]api.Introducer) {
	n, _ := 32, uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, _, peerConnectedAddrs, _, selfID := search.NewTestPeers(rng, n)
	apiSelf := &api.PeerAddress{
		PeerId: peers[0].ID().Bytes(),
	}
	intro := NewIntroduction(selfID, apiSelf, &Parameters{})
	introducers := make(map[string]api.Introducer)
	for address, connectedAddrs := range peerConnectedAddrs {
		introducers[address] = &fixedIntroducer{
			addresses: connectedAddrs,
		}
	}

	return intro, introducers
}

func newTestIntroducer(
	peersMap map[string]peer.Peer,
	selfID id.ID,
	peerConnectedAddrs map[string][]*api.PeerAddress,
	rec comm.QueryRecorder,
) Introducer {
	addressIntroducers := make(map[string]api.Introducer)
	for _, p := range peersMap {
		addressIntroducers[p.Address().String()] = &fixedIntroducer{
			addresses: peerConnectedAddrs[p.Address().String()],
			self:      p.ToAPI(),
		}
	}
	return NewIntroducer(
		&lclient.TestNoOpSigner{},
		rec,
		&fixedIntroducerCreator{introducers: addressIntroducers},
		&responseProcessor{
			fromer: &search.TestFromer{Peers: peersMap},
			selfID: selfID,
		},
	)
}

func newTestIntros(
	concurrency uint, rec comm.QueryRecorder,
) (Introducer, *Introduction, []int, []peer.Peer) {
	n, targetNumIntros := 256, uint(64)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, peerConnectedAddrs, selfPeerIdxs, selfID := search.NewTestPeers(rng, n)
	apiSelf := &api.PeerAddress{
		PeerId: peers[0].ID().Bytes(),
	}

	// create our introducer
	introducer := newTestIntroducer(peersMap, selfID.ID(), peerConnectedAddrs, rec)

	intro := NewIntroduction(selfID, apiSelf, &Parameters{
		TargetNumIntroductions: targetNumIntros,
		NumPeersPerRequest:     DefaultNumPeersPerRequest,
		NMaxErrors:             DefaultNMaxErrors,
		Concurrency:            concurrency,
		Timeout:                DefaultQueryTimeout,
	})

	return introducer, intro, selfPeerIdxs, peers
}

type fixedIntroducerCreator struct {
	introducers map[string]api.Introducer
	err         error
}

func (c *fixedIntroducerCreator) Create(address string) (api.Introducer, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.introducers[address], nil
}

type fixedIntroducer struct {
	self      *api.PeerAddress
	addresses []*api.PeerAddress
	requestID []byte
	err       error
}

func (f *fixedIntroducer) Introduce(ctx context.Context, rq *api.IntroduceRequest,
	opts ...grpc.CallOption) (*api.IntroduceResponse, error) {

	if f.err != nil {
		return nil, f.err
	}
	requestID := f.requestID
	if requestID == nil {
		requestID = rq.Metadata.RequestId
	}
	return &api.IntroduceResponse{
		Metadata: &api.ResponseMetadata{
			RequestId: requestID,
		},
		Self:  f.self,
		Peers: f.addresses,
	}, nil
}

type fixedRecorder struct {
	nSuccesses int
	nErrors    int
	mu         sync.Mutex
}

func (f *fixedRecorder) Record(
	peerID id.ID, endpoint api.Endpoint, qt comm.QueryType, o comm.Outcome,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if o == comm.Success {
		f.nSuccesses++
	} else {
		f.nErrors++
	}
}

func (f *fixedRecorder) Get(peerID id.ID, endpoint api.Endpoint) comm.QueryOutcomes {
	panic("implement me")
}

func (f *fixedRecorder) CountPeers(endpoint api.Endpoint, qt comm.QueryType, known bool) int {
	panic("implement me")
}
