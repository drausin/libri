package introduce

import (
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/signature"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/drausin/libri/libri/librarian/api"
	cid "github.com/drausin/libri/libri/common/id"
	"sync"
	"bytes"
	"fmt"
)


// Introducer executes recursive introductions.
type Introducer interface {
	Introduce(intro *Introduction, seeds []peer.Peer) error
}

type introducer struct {
	signer signature.Signer

	querier client.IntroduceQuerier

	repProcessor ResponseProcessor
}

func NewIntroducer(s signature.Signer, q client.IntroduceQuerier, rp ResponseProcessor) (
	Introducer) {
	return &introducer{
		signer: s,
		querier: q,
		repProcessor: rp,
	}
}

func NewDefaultIntroducer(s signature.Signer) Introducer {
	return NewIntroducer(
		s,
		client.NewIntroduceQuerier(),
		NewResponseProcessor(peer.NewFromer()),
	)
}

func (i *introducer) Introduce(intro *Introduction, seeds []peer.Peer) error {
	for _, seed := range seeds {
		intro.Result.Unqueried[seed.ID().String()] = seed
	}

	var wg sync.WaitGroup
	for c := uint(0); c < intro.Params.Concurrency; c++ {
		wg.Add(1)
		go i.introductionWork(intro, &wg)
	}
	wg.Wait()

	return intro.Result.FatalErr
}

func (i *introducer) introductionWork(intro *Introduction, wg *sync.WaitGroup) {
	defer wg.Done()
	for !intro.Finished() {

		// get next peer to query
		var nextIdStr string
		var next peer.Peer
		intro.WrapLock(func() {
			nextIdStr, next = getAny(intro.Result.Unqueried)
		})
		if _, err := next.Connector().Connect(); err != nil {
			// if we have issues connecting, skip to next peer
			continue
		}

		// do the query
		response, err := i.query(next.Connector(), intro)
		if err != nil {
			// if we had an issue querying, skip to next peer
			intro.WrapLock(func() {
				intro.Result.NErrors++
			})
			next.Recorder().Record(peer.Response, peer.Error)
			continue
		}
		next.Recorder().Record(peer.Response, peer.Success)

		// process the heap's response
		intro.WrapLock(func() {
			delete(intro.Result.Unqueried, nextIdStr)
			err = i.repProcessor.Process(response, intro.Result)
		})
		if err != nil {
			intro.WrapLock(func() {
				intro.Result.FatalErr = err
			})
			return
		}
	}
}


func (i *introducer) query(pConn peer.Connector, intro *Introduction) (*api.IntroduceResponse,
	error) {
	rq := intro.NewRequest()
	ctx, cancel, err := client.NewSignedTimeoutContext(i.signer, rq, intro.Params.Timeout)
	defer cancel()
	if err != nil {
		return nil, err
	}

	rp, err := i.querier.Query(ctx, pConn, rq)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(rp.Metadata.RequestId, rq.Metadata.RequestId) {
		return nil, fmt.Errorf("unexpected response request ID received: %v, "+
			"expected %v", rp.Metadata.RequestId, rq.Metadata.RequestId)
	}

	return rp, nil
}


func getAny(m map[string]peer.Peer) (string, peer.Peer) {
	for k, v := range m {
		return k, v
	}
	return "empty", nil
}


type ResponseProcessor interface {
	Process(*api.IntroduceResponse, *Result) error
}

type responseProcessor struct {
	fromer peer.Fromer
}

func NewResponseProcessor(f peer.Fromer) ResponseProcessor {
	return &responseProcessor{fromer: f}
}

func (irp *responseProcessor) Process(rp *api.IntroduceResponse, result *Result) error {

	// add newly introduced peer to responded map
	idStr := cid.FromBytes(rp.Self.PeerId).String()
	newPeer := irp.fromer.FromAPI(rp.Self)
	result.Responded[idStr] = newPeer

	// add newly discovered peers to list of peers to query if they're not already there
	for _, pa := range rp.Peers {
		newIdStr := cid.FromBytes(pa.PeerId).String()
		_, inResponded := result.Responded[newIdStr]
		_, inUnqueried := result.Unqueried[newIdStr]
		if !inResponded && !inUnqueried {
			newPeer := irp.fromer.FromAPI(rp.Self)
			result.Unqueried[newIdStr] = newPeer
		}
	}

	return nil
}