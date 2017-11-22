package verify

import (
	"bytes"
	"container/heap"
	"sync"
	"time"

	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/search"
	"github.com/pkg/errors"
)

const verifierVerifyRetryTimeout = 25 * time.Millisecond

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

type peerResponse struct {
	peer     peer.Peer
	response *api.VerifyResponse
	err      error
}

func (v *verifier) Verify(verify *Verify, seeds []peer.Peer) error {
	toQuery := search.NewQueryQueue()
	peerResponses := make(chan *peerResponse, 1)

	// add seeds and queue some of them for querying
	err := verify.Result.Unqueried.SafePushMany(seeds)
	cerrors.MaybePanic(err)

	go func() {
		for c := uint(0); c < verify.Params.Concurrency; c++ {
			if next := getNextToQuery(verify); next != nil {
				toQuery.MaybeSend(next)
			}
		}
	}()

	// goroutine that processes responses and queues up next peer to query
	var wg1 sync.WaitGroup
	wg1.Add(1)
	go func(wg2 *sync.WaitGroup) {
		defer wg2.Done()
		for pr := range peerResponses {
			processAnyReponse(pr, v.rp, verify)
			maybeSendNextToQuery(toQuery, verify)
		}
	}(&wg1)

	var wg3 sync.WaitGroup
	for c := uint(0); c < verify.Params.Concurrency; c++ {
		wg3.Add(1)
		go func(wg4 *sync.WaitGroup) {
			defer wg4.Done()
			for next := range toQuery.Peers {
				verify.AddQueried(next)
				response, err := v.query(next.Connector(), verify)
				peerResponses <- &peerResponse{
					peer:     next,
					response: response,
					err:      err,
				}
			}
		}(&wg3)
	}
	wg3.Wait()
	close(peerResponses)

	wg1.Wait()
	return verify.Result.FatalErr
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

func getNextToQuery(verify *Verify) peer.Peer {
	if verify.Finished() || verify.Exhausted() {
		return nil
	}
	verify.mu.Lock()
	defer verify.mu.Unlock()
	next := heap.Pop(verify.Result.Unqueried).(peer.Peer)
	if _, alreadyQueried := verify.Result.Queried[next.ID().String()]; alreadyQueried {
		return nil
	}
	return next
}

func maybeSendNextToQuery(toQuery *search.QueryQueue, verify *Verify) {
	var exhausted bool
	verify.wrapLock(func() {
		exhausted = verify.Exhausted()
	})
	if stopQuerying := verify.Finished() || exhausted; stopQuerying {
		toQuery.MaybeClose()
	}
	if next := getNextToQuery(verify); next != nil {
		toQuery.MaybeSend(next)
	}
}

func processAnyReponse(pr *peerResponse, rp ResponseProcessor, verify *Verify) {
	if pr.err != nil {
		recordError(pr.peer, pr.err, verify)
	} else if err := rp.Process(pr.response, pr.peer, verify.ExpectedMAC, verify); err != nil {
		recordError(pr.peer, err, verify)
	} else {
		recordSuccess(pr.peer, verify)
	}
}

func recordError(p peer.Peer, err error, v *Verify) {
	v.wrapLock(func() {
		v.Result.Errored[p.ID().String()] = err
		if v.Errored() {
			v.Result.FatalErr = errTooManyVerifyErrors
		}
	})
	p.Recorder().Record(peer.Response, peer.Error)
}

func recordSuccess(p peer.Peer, v *Verify) {
	v.wrapLock(func() {
		v.Result.Responded[p.ID().String()] = p
	})
	p.Recorder().Record(peer.Response, peer.Success)
}

// ResponseProcessor handles api.VerifyResponses.
type ResponseProcessor interface {
	// Process handles an api.VerifyResponse, adding newly discovered peers to the unqueried
	// ClosestPeers heap and adding from to the list of replicas when its MAC matches the expected
	// value.
	Process(response *api.VerifyResponse, from peer.Peer, expectedMAC []byte, verify *Verify) error
}

type responseProcessor struct {
	fromer peer.Fromer
}

// NewResponseProcessor creates a new ResponseProcessor.
func NewResponseProcessor(fromer peer.Fromer) ResponseProcessor {
	return &responseProcessor{fromer: fromer}
}

func (vrp *responseProcessor) Process(
	rp *api.VerifyResponse, from peer.Peer, expectedMAC []byte, v *Verify,
) error {
	if rp.Mac != nil {
		// peer claims to have the value
		if bytes.Equal(expectedMAC, rp.Mac) {
			// they do!
			v.Result.Replicas[from.ID().String()] = from
			return nil
		}
		// they don't
		return errUnexpectedVerifyMAC
	}

	if rp.Peers != nil {
		// response has peer addresses close to key
		v.wrapLock(func() {
			err := v.Result.Closest.SafePush(from)
			cerrors.MaybePanic(err) // should never happen
		})
		if !v.Finished() {
			// don't add peers to unqueried if the verify is already finished and we're not going
			// to query those peers; there's a small chance that one of the peer that would be added
			// to unqueried would be closer than farthest closest peer, which would be then move the
			// verify state back from finished -> not finished, a potentially confusing change
			// that we choose to avoid altogether at the expense of (very) occasionally missing a
			// closer peer
			v.wrapLock(func() {
				search.AddPeers(v.Result.Queried, v.Result.Unqueried, rp.Peers, vrp.fromer)
			})
		}
		return nil
	}

	// invalid response
	return errInvalidResponse
}
