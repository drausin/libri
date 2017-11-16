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
	toQuery := make(chan peer.Peer, verify.Params.Concurrency)
	peerResponses := make(chan *peerResponse, verify.Params.Concurrency)

	// add seeds and queue some of them for querying
	err := verify.Result.Unqueried.SafePushMany(seeds)
	cerrors.MaybePanic(err)
	for c := uint(0); c < verify.Params.Concurrency; c++ {
		next := getNextToQuery(verify)
		if next != nil {
			toQuery <- next
		}
	}

	// goroutine that processes responses and queues up next peer to query
	go func() {
		for peerResponse := range peerResponses {
			if finished := processAnyReponse(peerResponse, v.rp, verify); finished {
				break
			}
			if finished := sendNextToQuery(toQuery, verify); finished {
				break
			}
		}
		verify.wrapLock(func() { maybeClose(toQuery) })
	}()

	var wg1 sync.WaitGroup
	for c := uint(0); c < verify.Params.Concurrency; c++ {
		wg1.Add(1)
		go func(wg2 *sync.WaitGroup) {
			defer wg2.Done()
			for next := range toQuery {
				if verify.Finished() {
					break
				}
				verify.AddQueried(next)
				response, err := v.query(next.Connector(), verify)
				peerResponses <- &peerResponse{
					peer:     next,
					response: response,
					err:      err,
				}
			}
			verify.wrapLock(func() { maybeClose(toQuery) })
		}(&wg1)
	}

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

func maybeClose(toQuery chan peer.Peer) {
	select {
	case <-toQuery:
	default:
		close(toQuery)
	}
}

func getNextToQuery(verify *Verify) peer.Peer {
	if verify.Finished() {
		return nil
	}
	var next peer.Peer
	var alreadyQueried bool
	verify.wrapLock(func() {
		next = heap.Pop(verify.Result.Unqueried).(peer.Peer)
		_, alreadyQueried = verify.Result.Queried[next.ID().String()]
	})
	if alreadyQueried {
		return nil
	}
	return next
}

func sendNextToQuery(toQuery chan peer.Peer, verify *Verify) bool {
	next := getNextToQuery(verify)
	if next != nil {
		verify.wrapLock(func() {
			select {
			case <-toQuery: // already closed
			case toQuery <- next:
			}
		})
	}
	return verify.Finished()
}

func processAnyReponse(pr *peerResponse, rp ResponseProcessor, verify *Verify) bool {

	if pr.err != nil {
		recordError(pr.peer, pr.err, verify)
	} else if err := rp.Process(pr.response, pr.peer, verify.ExpectedMAC, verify); err != nil {
		recordError(pr.peer, err, verify)
	} else {
		recordSuccess(pr.peer, verify)
	}
	return verify.Finished()
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
	rp *api.VerifyResponse, from peer.Peer, expectedMAC []byte, verify *Verify,
) error {
	if rp.Mac != nil {
		// peer claims to have the value
		if bytes.Equal(expectedMAC, rp.Mac) {
			// they do!
			verify.Result.Replicas[from.ID().String()] = from
			return nil
		}
		// they don't
		return errUnexpectedVerifyMAC
	}

	if rp.Peers != nil {
		// response has peer addresses close to key
		verify.wrapLock(func() {
			search.AddPeers(verify.Result.Queried, verify.Result.Unqueried, rp.Peers, vrp.fromer)
			err := verify.Result.Closest.SafePush(from)
			cerrors.MaybePanic(err) // should never happen
		})
		return nil
	}

	// invalid response
	return errInvalidResponse
}
