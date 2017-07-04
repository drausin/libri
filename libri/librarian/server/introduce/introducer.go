package introduce

import (
	"bytes"
	"fmt"
	"sync"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

// Introducer executes recursive introductions.
type Introducer interface {
	// Introduce executes an introduction from a list of seeds.
	Introduce(intro *Introduction, seeds []peer.Peer) error
}

type introducer struct {
	signer       client.Signer
	querier      client.IntroduceQuerier
	repProcessor ResponseProcessor
}

// NewIntroducer creates a new Introducer instance with the given signer, querier, and response
// processor.
func NewIntroducer(s client.Signer, q client.IntroduceQuerier, rp ResponseProcessor) Introducer {
	return &introducer{
		signer:       s,
		querier:      q,
		repProcessor: rp,
	}
}

// NewDefaultIntroducer creates a new Introducer with the given signer and default querier and
// response processor.
func NewDefaultIntroducer(s client.Signer, selfID cid.ID) Introducer {
	return NewIntroducer(
		s,
		client.NewIntroduceQuerier(),
		NewResponseProcessor(peer.NewFromer(), selfID),
	)
}

func (i *introducer) Introduce(intro *Introduction, seeds []peer.Peer) error {
	for i, seed := range seeds {
		// since we may be bootstrapping, these peers may not have IDs, so create our own
		// (temporary) ID strings
		seedIDStr := fmt.Sprintf("seed%02d", i)
		intro.Result.Unqueried[seedIDStr] = seed
	}

	var wg sync.WaitGroup
	for c := uint(0); c < intro.Params.Concurrency; c++ {
		wg.Add(1)
		go i.introduceWork(intro, &wg)
	}
	wg.Wait()

	return intro.Result.FatalErr
}

func (i *introducer) introduceWork(intro *Introduction, wg *sync.WaitGroup) {
	defer wg.Done()
	for !intro.Finished() {

		// get next peer to query
		var nextIDStr string
		var next peer.Peer
		intro.wrapLock(func() { nextIDStr, next = removeAny(intro.Result.Unqueried) })
		if next == nil {
			// no more unqueried peers
			continue
		}

		// do the query
		response, err := i.query(next.Connector(), intro)
		if err != nil {
			// if we had an issue querying, skip to next peer
			intro.wrapLock(func() {
				intro.Result.NErrors++
				next.Recorder().Record(peer.Response, peer.Error)
			})
			continue
		}
		intro.wrapLock(func() { next.Recorder().Record(peer.Response, peer.Success) })

		// process the heap's response
		intro.wrapLock(func() {
			delete(intro.Result.Unqueried, nextIDStr)
			err = i.repProcessor.Process(response, intro.Result)
		})
		if err != nil {
			intro.wrapLock(func() { intro.Result.FatalErr = err })
			return
		}
		err = next.Connector().Disconnect()
		if err != nil {
			intro.wrapLock(func() { intro.Result.FatalErr = err })
			return
		}
	}
}

func (i *introducer) query(pConn api.Connector, intro *Introduction) (*api.IntroduceResponse,
	error) {
	rq := intro.NewRequest()
	ctx, cancel, err := client.NewSignedTimeoutContext(i.signer, rq, intro.Params.Timeout)
	if err != nil {
		return nil, err
	}
	rp, err := i.querier.Query(ctx, pConn, rq)
	cancel()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, rq.Metadata.RequestId) {
		return nil, client.ErrUnexpectedRequestID
	}

	return rp, nil
}

func removeAny(m map[string]peer.Peer) (string, peer.Peer) {
	for k, v := range m {
		delete(m, k)
		return k, v
	}
	return "empty", nil
}

// ResponseProcessor handles an api.IntroduceResponse.
type ResponseProcessor interface {
	// Process handles an api.IntroduceResponse, adding the responder to the map of responded
	// peers and newly discovered peers to the unqueried map.
	Process(*api.IntroduceResponse, *Result) error
}

type responseProcessor struct {
	fromer peer.Fromer
	selfID cid.ID
}

// NewResponseProcessor creates a new ResponseProcessor with a given peer.Fromer.
func NewResponseProcessor(f peer.Fromer, selfID cid.ID) ResponseProcessor {
	return &responseProcessor{
		fromer: f,
		selfID: selfID,
	}
}

func (irp *responseProcessor) Process(rp *api.IntroduceResponse, result *Result) error {

	// add newly introduced peer to responded map
	idStr := cid.FromBytes(rp.Self.PeerId).String()
	newPeer := irp.fromer.FromAPI(rp.Self)
	result.Responded[idStr] = newPeer

	// add newly discovered peers to list of peers to query if they're not already there
	selfIDStr := irp.selfID.String()
	for _, pa := range rp.Peers {
		newIDStr := cid.FromBytes(pa.PeerId).String()
		_, inResponded := result.Responded[newIDStr]
		_, inUnqueried := result.Unqueried[newIDStr]
		if !inResponded && !inUnqueried && newIDStr != selfIDStr {
			newPeer := irp.fromer.FromAPI(pa)
			result.Unqueried[newIDStr] = newPeer
		}
	}

	return nil
}
