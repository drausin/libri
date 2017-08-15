package verify

import (
	"bytes"
	"sync"
	"time"

	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/pkg/errors"
)

const verifierVerifyRetryTimeout = 100 * time.Millisecond

var (
	errInvalidResponse     = errors.New("VerifyResponse contains neither MAC nor peer addresses")
	errTooManyVerifyErrors = errors.New("too many Verify errors")
	errUnexpectedVerifyMAC = errors.New("unexpected Verify MAC")
)

// Verifier executes verifications for particular keys.
type Verifier interface {
	// Verify executes the verification.
	Verify(verify *Verify, seeds []peer.Peer) error
}

type verifier struct {
	signer client.Signer

	verifierCreator client.VerifierCreator

	rp ResponseProcessor
}

// NewVerifier returns a new Verifier with the given Querier and ResponseProcessor.
func NewVerifier(s client.Signer, c client.VerifierCreator, rp ResponseProcessor) Verifier {
	return &verifier{
		signer:          s,
		verifierCreator: c,
		rp:              rp,
	}
}

// NewDefaultVerifier creates a new Verifier with default sub-object instantiations.
func NewDefaultVerifier(signer client.Signer) Verifier {
	return NewVerifier(
		signer,
		client.NewVerifierCreator(),
		NewResponseProcessor(peer.NewFromer()),
	)
}

func (v *verifier) Verify(verify *Verify, seeds []peer.Peer) error {
	cerrors.MaybePanic(verify.Result.Unqueried.SafePushMany(seeds))

	var wg sync.WaitGroup
	for c := uint(0); c < verify.Params.Concurrency; c++ {
		wg.Add(1)
		go v.verifyWork(verify, &wg)
	}
	wg.Wait()

	return verify.Result.FatalErr
}

func (v *verifier) verifyWork(verify *Verify, wg *sync.WaitGroup) {
	defer wg.Done()
	result := verify.Result

	for !verify.Finished() {

		// get next peer to query
		verify.mu.Lock()
		next, nextIDStr := search.NextPeerToQuery(
			result.Unqueried,
			result.Responded,
			result.Errored,
		)
		verify.mu.Unlock()
		if next == nil {
			// next peer has already responded or errored
			continue
		}

		// do the query
		response, err := v.query(next.Connector(), verify)
		if err != nil {
			// if we had an issue querying, skip to next peer
			verify.wrapLock(func() { recordError(nextIDStr, next, err, verify) })
			continue
		}

		// process the response
		verify.wrapLock(func() { err = v.rp.Process(response, next, verify.ExpectedMAC, result) })
		if err != nil {
			verify.wrapLock(func() { recordError(nextIDStr, next, err, verify) })
			continue
		}

		// bookkeep ok response
		verify.wrapLock(func() {
			next.Recorder().Record(peer.Response, peer.Success)
			result.Responded[nextIDStr] = next
		})

	}
}

func recordError(nextIDStr string, p peer.Peer, err error, v *Verify) {
	v.Result.Errored[nextIDStr] = err
	p.Recorder().Record(peer.Response, peer.Error)
	if v.Errored() {
		v.Result.FatalErr = errTooManyVerifyErrors
	}
}

func (v *verifier) query(pConn peer.Connector, verify *Verify) (*api.VerifyResponse, error) {
	verifyClient, err := v.verifierCreator.Create(pConn)
	if err != nil {
		return nil, err
	}
	rq := verify.RequestCreator()
	ctx, cancel, err := client.NewSignedTimeoutContext(v.signer, rq, verify.Params.Timeout)
	if err != nil {
		return nil, err
	}
	retryVerifyClient := client.NewRetryVerifier(verifyClient, verifierVerifyRetryTimeout)
	rp, err := retryVerifyClient.Verify(ctx, rq)
	cancel()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, rq.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}

	return rp, nil
}

// ResponseProcessor handles api.VerifyResponses.
type ResponseProcessor interface {
	// Process handles an api.VerifyResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap and adding from to the list of replicas when its MAC matches the expected
	// value.
	Process(response *api.VerifyResponse, from peer.Peer, expectedMAC []byte, result *Result) error
}

type responseProcessor struct {
	fromer peer.Fromer
}

// NewResponseProcessor creates a new ResponseProcessor.
func NewResponseProcessor(fromer peer.Fromer) ResponseProcessor {
	return &responseProcessor{fromer: fromer}
}

func (vrp *responseProcessor) Process(
	rp *api.VerifyResponse, from peer.Peer, expectedMAC []byte, result *Result,
) error {
	if rp.Mac != nil {
		// peer claims to have the value
		if bytes.Equal(expectedMAC, rp.Mac) {
			// they do!
			result.Replicas[from.ID().String()] = from
			return nil
		}
		// they don't
		return errUnexpectedVerifyMAC
	}

	if rp.Peers != nil {
		// response has peer addresses close to key
		search.AddPeers(result.Responded, result.Unqueried, rp.Peers, vrp.fromer)
		cerrors.MaybePanic(result.Closest.SafePush(from))
		return nil
	}

	// invalid response
	return errInvalidResponse
}
