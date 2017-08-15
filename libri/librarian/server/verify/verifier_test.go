package verify

import (
	"container/heap"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestNewDefaultVerifier(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	v := NewDefaultVerifier(client.NewSigner(ecid.NewPseudoRandom(rng).Key()))
	assert.NotNil(t, v.(*verifier).signer)
	assert.NotNil(t, v.(*verifier).verifierCreator)
	assert.NotNil(t, v.(*verifier).rp)
}

func TestVerifier_Verify_ok(t *testing.T) {
	n, nReplicas, nClosestResponses := 32, uint(3), uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := search.NewTestPeers(rng, n)
	macKey, mac := api.RandBytes(rng, 32), api.RandBytes(rng, 32)

	// create our verifier
	key := id.NewPseudoRandom(rng)
	verifier := newTestVerifier(peersMap)

	for concurrency := uint(1); concurrency <= 3; concurrency++ {

		v := NewVerify(selfID, key, macKey, mac, &Parameters{
			NReplicas:         nReplicas,
			NClosestResponses: nClosestResponses,
			NMaxErrors:        search.DefaultNMaxErrors,
			Concurrency:       concurrency,
			Timeout:           search.DefaultQueryTimeout,
		})
		seeds := search.NewTestSeeds(peers, selfPeerIdxs)

		// verify!
		err := verifier.Verify(v, seeds)

		// checks
		assert.Nil(t, err)
		assert.True(t, v.Finished())
		assert.True(t, v.UnderReplicated())
		assert.False(t, v.Errored())
		assert.False(t, v.Exhausted())
		assert.Equal(t, 0, len(v.Result.Errored))
		assert.Equal(t, int(nClosestResponses), v.Result.Closest.Len())
		assert.True(t, v.Result.Closest.Len() <= len(v.Result.Responded))

		// build set of closest peers by iteratively looking at all of them
		expectedClosestsPeers := make(map[string]struct{})
		farthestCloseDist := v.Result.Closest.PeakDistance()
		for _, p := range peers {
			pDist := key.Distance(p.ID())
			if pDist.Cmp(farthestCloseDist) <= 0 {
				expectedClosestsPeers[p.ID().String()] = struct{}{}
			}
		}

		// check all closest peers are in set of peers within farther close distance to the key
		for v.Result.Closest.Len() > 0 {
			p := heap.Pop(v.Result.Closest).(peer.Peer)
			_, in := expectedClosestsPeers[p.ID().String()]
			assert.True(t, in)
		}
	}
}

func TestVerifier_Verify_queryErr(t *testing.T) {
	verifierImpl, verify, selfPeerIdxs, peers := newTestVerify()
	seeds := search.NewTestSeeds(peers, selfPeerIdxs)

	// duplicate seeds so we cover branch of hitting errored peer more than once
	seeds = append(seeds, seeds[0])

	// all queries return errors
	verifierImpl.(*verifier).verifierCreator = &testVerifierCreator{
		err: errors.New("some Create error"),
	}

	// do the verify!
	err := verifierImpl.Verify(verify, seeds)

	// checks
	assert.Equal(t, errTooManyVerifyErrors, err)
	assert.True(t, verify.Errored())    // since all of the queries return errors
	assert.False(t, verify.Exhausted()) // since NMaxErrors < len(Unqueried)
	assert.True(t, verify.Finished())
	assert.False(t, verify.UnderReplicated())
	assert.Equal(t, int(verify.Params.NMaxErrors+1), len(verify.Result.Errored))
	assert.Equal(t, 0, len(verify.Result.Replicas))
	assert.Equal(t, 0, verify.Result.Closest.Len())
	assert.True(t, 0 < verify.Result.Unqueried.Len())
	assert.Equal(t, 0, len(verify.Result.Responded))
}

func TestVerifier_Verify_rpErr(t *testing.T) {
	verifierImpl, verify, selfPeerIdxs, peers := newTestVerify()
	seeds := search.NewTestSeeds(peers, selfPeerIdxs)

	// mock some internal issue when processing responses
	verifierImpl.(*verifier).rp = &errResponseProcessor{}

	// do the verify!
	err := verifierImpl.Verify(verify, seeds)

	// checks
	assert.NotNil(t, err)
	assert.NotNil(t, verify.Result.FatalErr)
	assert.True(t, verify.Errored()) // since we hit max number of allowable errors
	assert.False(t, verify.Exhausted())
	assert.True(t, verify.Finished())
	assert.False(t, verify.UnderReplicated())
	assert.Equal(t, int(verify.Params.NMaxErrors+1), len(verify.Result.Errored))
	assert.Equal(t, 0, verify.Result.Closest.Len())
	assert.True(t, 0 < verify.Result.Unqueried.Len())
	assert.Equal(t, 0, len(verify.Result.Responded))
}

func TestVerifier_query_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	macKey, mac := api.RandBytes(rng, 32), api.RandBytes(rng, 32)

	v := NewVerify(peerID, key, macKey, mac, &Parameters{})
	verifierImpl := &verifier{
		signer:          &client.TestNoOpSigner{},
		verifierCreator: &testVerifierCreator{},
		rp:              nil,
	}
	connClient := &peer.TestConnector{}

	rp, err := verifierImpl.query(connClient, v)
	assert.Nil(t, err)
	assert.NotNil(t, rp.Metadata.RequestId)
	assert.Nil(t, rp.Mac)
}

func TestVerifier_query_err(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	connClient := &peer.TestConnector{}
	peerID, key := ecid.NewPseudoRandom(rng), id.NewPseudoRandom(rng)
	macKey, mac := api.RandBytes(rng, 32), api.RandBytes(rng, 32)

	v := NewVerify(peerID, key, macKey, mac, &Parameters{Timeout: 1 * time.Second})

	cases := []*verifier{
		// case 0
		{
			signer:          &client.TestNoOpSigner{},
			verifierCreator: &testVerifierCreator{err: errors.New("some create error")},
		},

		// case 1
		{
			signer:          &client.TestErrSigner{},
			verifierCreator: &testVerifierCreator{},
		},

		// case 2
		{
			signer: &client.TestNoOpSigner{},
			verifierCreator: &testVerifierCreator{
				verifier: &fixedVerifier{err: errors.New("some Find error")},
			},
		},

		// case 3
		{
			signer: &client.TestNoOpSigner{},
			verifierCreator: &testVerifierCreator{
				verifier: &fixedVerifier{requestID: []byte{1, 2, 3, 4}},
			},
		},
	}

	for i, c := range cases {
		info := fmt.Sprintf("case %d", i)
		rp, err := c.query(connClient, v)
		assert.Nil(t, rp, info)
		assert.NotNil(t, err, info)
	}
}

func TestResponseProcessor_Process_MAC(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	key := id.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	result := NewInitialResult(key, NewDefaultParameters())
	expectedMAC := api.RandBytes(rng, 32)
	from := peer.NewTestPeer(rng, 0)

	// check that peer is added to replicas
	response1 := &api.VerifyResponse{Mac: expectedMAC}
	err := rp.Process(response1, from, expectedMAC, result)
	assert.Nil(t, err)
	assert.Zero(t, result.Unqueried.Len())
	assert.Equal(t, 1, len(result.Replicas))

	// check error on unexpected MAC
	response2 := &api.VerifyResponse{Mac: api.RandBytes(rng, 32)}
	err = rp.Process(response2, from, expectedMAC, result)
	assert.Equal(t, errUnexpectedVerifyMAC, err)

	// check error on invalid response
	response3 := &api.VerifyResponse{}
	err = rp.Process(response3, from, expectedMAC, result)
	assert.Equal(t, errInvalidResponse, err)
}

func TestResponseProcessor_Process_Addresses(t *testing.T) {
	rng := rand.New(rand.NewSource(int64(0)))
	nAddresses := 6
	key := id.NewPseudoRandom(rng)
	rp := NewResponseProcessor(peer.NewFromer())
	params := NewDefaultParameters()
	result := NewInitialResult(key, params)
	result.Unqueried = search.NewClosestPeers(key, 9)
	expectedMAC := api.RandBytes(rng, 32)
	from := peer.NewTestPeer(rng, 0)
	peerAddresses := newPeerAddresses(rng, nAddresses)

	// create response or nAddresses and process it
	response := &api.VerifyResponse{
		Peers: peerAddresses,
	}
	err := rp.Process(response, from, expectedMAC, result)
	assert.Nil(t, err)
	assert.Equal(t, nAddresses, result.Unqueried.Len())
	assert.True(t, result.Closest.In(from.ID()))
	assert.Equal(t, 1, result.Closest.Len())
}

func newTestVerify() (Verifier, *Verify, []int, []peer.Peer) {
	n, nReplicas, nClosestResponses := 32, uint(3), uint(8)
	rng := rand.New(rand.NewSource(int64(n)))
	peers, peersMap, selfPeerIdxs, selfID := search.NewTestPeers(rng, n)
	macKey, mac := api.RandBytes(rng, 32), api.RandBytes(rng, 32)

	// create our verifier
	key := id.NewPseudoRandom(rng)
	verifier := newTestVerifier(peersMap)

	v := NewVerify(selfID, key, macKey, mac, &Parameters{
		NReplicas:         nReplicas,
		NClosestResponses: nClosestResponses,
		NMaxErrors:        search.DefaultNMaxErrors,
		Concurrency:       uint(1),
		Timeout:           search.DefaultQueryTimeout,
	})

	return verifier, v, selfPeerIdxs, peers
}

func newTestVerifier(peersMap map[string]peer.Peer) Verifier {
	return NewVerifier(
		&client.TestNoOpSigner{},
		&testVerifierCreator{},
		&responseProcessor{
			fromer: &search.TestFromer{Peers: peersMap},
		},
	)
}

type errResponseProcessor struct{}

func (erp *errResponseProcessor) Process(
	rp *api.VerifyResponse, from peer.Peer, expectedMAC []byte, result *Result,
) error {
	return errors.New("some processing error")
}

type testVerifierCreator struct {
	verifier api.Verifier
	err      error
}

func (c *testVerifierCreator) Create(pConn peer.Connector) (api.Verifier, error) {
	if c.err != nil {
		return nil, c.err
	}
	if c.verifier != nil {
		return c.verifier, nil
	}
	return &fixedVerifier{addresses: pConn.(*peer.TestConnector).Addresses}, nil
}

type fixedVerifier struct {
	addresses []*api.PeerAddress
	requestID []byte
	err       error
}

func (f *fixedVerifier) Verify(
	ctx context.Context, rq *api.VerifyRequest, opts ...grpc.CallOption,
) (*api.VerifyResponse, error) {

	if f.err != nil {
		return nil, f.err
	}
	requestID := f.requestID
	if requestID == nil {
		requestID = rq.Metadata.RequestId
	}
	return &api.VerifyResponse{
		Metadata: &api.ResponseMetadata{RequestId: requestID},
		Peers:    f.addresses,
	}, nil
}

func newPeerAddresses(rng *rand.Rand, n int) []*api.PeerAddress {
	peerAddresses := make([]*api.PeerAddress, n)
	for i, p := range peer.NewTestPeers(rng, n) {
		peerAddresses[i] = p.ToAPI()
	}
	return peerAddresses
}
