package introduce

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/peer"
)

// Introducer executes recursive introductions.
type Introducer interface {
	// Introduce executes an introduction from a list of seeds.
	Introduce(intro *Introduction, seeds []peer.Peer) error
}

type introducer struct {
	signer            client.Signer
	introducerCreator client.IntroducerCreator
	repProcessor      ResponseProcessor
	rec               comm.QueryRecorder
}

// NewIntroducer creates a new Introducer instance with the given signer, querier, and response
// processor.
func NewIntroducer(
	s client.Signer, rec comm.QueryRecorder, c client.IntroducerCreator, rp ResponseProcessor,
) Introducer {
	return &introducer{
		signer:            s,
		introducerCreator: c,
		repProcessor:      rp,
		rec:               rec,
	}
}

// NewDefaultIntroducer creates a new Introducer with the given signer and default querier and
// response processor.
func NewDefaultIntroducer(
	s client.Signer, rec comm.QueryRecorder, selfID id.ID, clients client.Pool,
) Introducer {
	ic := client.NewIntroducerCreator(clients)
	rp := NewResponseProcessor(peer.NewFromer(), selfID)
	return NewIntroducer(s, rec, ic, rp)
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
		rp, err := i.query(next, intro)
		if err != nil {
			// if we had an issue querying, skip to next peer
			intro.mu.Lock()
			intro.Result.NErrors++
			if next.ID() != nil {
				i.rec.Record(next.ID(), api.Introduce, comm.Response, comm.Error)
			}
			intro.mu.Unlock()
			continue
		}

		// process the heap's rp
		intro.wrapLock(func() {
			delete(intro.Result.Unqueried, nextIDStr)
			i.repProcessor.Process(rp, intro.Result)
		})
		i.rec.Record(id.FromBytes(rp.Self.PeerId), api.Introduce, comm.Response, comm.Success)
	}
}

func (i *introducer) query(next peer.Peer, intro *Introduction) (*api.IntroduceResponse, error) {
	lc, err := i.introducerCreator.Create(next.Address().String())
	if err != nil {
		return nil, err
	}
	rq := intro.NewRequest()
	ctx, cancel, err := client.NewSignedTimeoutContext(i.signer, rq, intro.Params.Timeout)
	if err != nil {
		return nil, err
	}
	rp, err := lc.Introduce(ctx, rq)
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
	Process(*api.IntroduceResponse, *Result)
}

type responseProcessor struct {
	fromer peer.Fromer
	selfID id.ID
}

// NewResponseProcessor creates a new ResponseProcessor with a given peer.Fromer.
func NewResponseProcessor(f peer.Fromer, selfID id.ID) ResponseProcessor {
	return &responseProcessor{
		fromer: f,
		selfID: selfID,
	}
}

func (irp *responseProcessor) Process(rp *api.IntroduceResponse, result *Result) {

	// add newly introduced peer to responded map
	idStr := id.FromBytes(rp.Self.PeerId).String()
	newPeer := irp.fromer.FromAPI(rp.Self)
	result.Responded[idStr] = newPeer

	// add newly discovered peers to list of peers to query if they're not already there
	selfIDStr := irp.selfID.String()
	for _, pa := range rp.Peers {
		newIDStr := id.FromBytes(pa.PeerId).String()
		_, inResponded := result.Responded[newIDStr]
		_, inUnqueried := result.Unqueried[newIDStr]
		if !inResponded && !inUnqueried && newIDStr != selfIDStr {
			newPeer := irp.fromer.FromAPI(pa)
			result.Unqueried[newIDStr] = newPeer
		}
	}
}
