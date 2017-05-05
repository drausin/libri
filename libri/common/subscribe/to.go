package subscribe

import (
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/pkg/errors"
	"io"
	"math/rand"
	"time"
	"sync"
)

// ErrTooManySubscriptionErrs indicates when too many subscription errors have occurred.
var ErrTooManySubscriptionErrs = errors.New("too many subscription errors")

const (
	/*
	 * The following table gives the consistencies (% of publications seen) when maintaining
	 * NSubscriptions with the FPRate. These are calculated via
	 *
	 *	consistency = 1 - BinomialCDF(x = 0 | p = FPRate, n = NSubscriptions)
	 *
	 *   FPRate	NSubscriptions	consistency
	 *   --------------------------------------------------
	 *   0.9	5		99.999%
	 *   0.9	10		99.9999999%
	 *
	 *   0.75	5		99.9%
	 *   0.75	10		99.9999%
	 *
	 *   0.5	10		99.9%
	 *   0.5	20		99.9999%
	 *
	 *   0.3	20		99.9%
	 *   0.3	40		99.9999%
	 */

	// DefaultNSubscriptionsTo is the default number of subscriptions to other peers to maintain.
	DefaultNSubscriptionsTo = 10

	// DefaultFPRate is the default false positive rate for each subscription to another peer.
	DefaultFPRate = 0.75

	// DefaultTimeout is the default timeout for Subscribe requests.
	DefaultTimeout = 5 * time.Second

	// DefaultMaxErrRate is the default maximum allowed error rate for Subscribe requests and
	// received publications before a fatal error is thrown.
	DefaultMaxErrRate = 0.1

	// DefaultRecentCacheSize is the default recent publications LRU cache size.
	DefaultRecentCacheSize = 1 << 12

	// errQueueSize is the size of the error queue used to calculate the running error rate.
	errQueueSize = 100
)

// ToParameters define how the collection of subscriptions to other peers will be managed.
type ToParameters struct {
	// NSubscriptions is the number of concurrent subscriptions maintained to other peers.
	NSubscriptions uint32

	// FPRate is the estimated false positive rate of the subscriptions.
	FPRate         float32

	// Timeout is the timeout for each Subscribe request.
	Timeout        time.Duration

	// MaxErrRate is the maximum allowed error rate for Subscribe requests and received
	// publications before a fatal error is thrown. This value is a running rate over a constant
	// history of responses (c.f., errQueueSize).
	MaxErrRate     float32

	// NRecentCacheSize is the size of the LRU cache used in deduplicating and grouping
	// publications.
	RecentCacheSize uint32
}

// NewDefaultParameters returns a *ToParameters object with default values.
func NewDefaultToParameters() *ToParameters {
	return &ToParameters{
		NSubscriptions: DefaultNSubscriptionsTo,
		FPRate: DefaultFPRate,
		Timeout: DefaultTimeout,
		MaxErrRate: DefaultMaxErrRate,
		RecentCacheSize: DefaultRecentCacheSize,
	}
}

// To maintains active subscriptions to a collection of peers, merging their publications into a
// single, deduplicated stream.
type To interface {
	// Begin starts and runs the subscriptions to the peers. It runs indefinitely until either
	// a fatal error is encountered, or the subscriptions are gracefully stopped via End().
	Begin() error

	// End gracefully stops the active subscriptions and closes the new channel
	End()
}

type to struct {
	params   *ToParameters
	cb       api.ClientBalancer
	sb subscriptionBeginner
	recent   RecentPublications
	received chan *pubValueReceipt
	new      chan *KeyedPub
	end      chan struct{}
}

// NewTo creates a new To instance, writing merged, deduplicated publications to the given new
// channel.
func NewTo(
	params *ToParameters,
	clientID ecid.ID,
	cb api.ClientBalancer,
	signer client.Signer,
	recent RecentPublications,
	new chan *KeyedPub,
) To {
	return &to{
		params: params,
		cb: cb,
		sb: &subscriptionBeginnerImpl{
			clientID: clientID,
			signer: signer,
			params: params,
		},
		recent: recent,
		received: make(chan *pubValueReceipt, params.NSubscriptions),
		new: new,
		end: make(chan struct{}),
	}
}

func (t *to) Begin() error {
	channelsSlack := t.params.NSubscriptions
	errs := make(chan error, channelsSlack)                // non-fatal errs and nils
	fatal := make(chan error)                              // signals fatal end

	// dedup all received publications, writing new to t.new & t.recent
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		t.dedup()
	}(wg)

	// monitor non-fatal errors, sending fatal err if too many
	go monitorRunningErrorCount(errs, fatal, t.params.MaxErrRate)

	// subscription threads writing to received & errs channels
	for c := uint32(0); c < t.params.NSubscriptions; c++ {
		go func(i uint32) {
			rng, fp := rand.New(rand.NewSource(int64(i))), float64(t.params.FPRate)
			for {
				lc, err := t.cb.Next()
				if err != nil {
					fatal <- err
					return
				}
				sub, err := NewFPSubscription(fp, rng)
				if err != nil {
					fatal <- err
					return
				}
				select {
				case <-t.end:
					return
				case errs <- t.sb.begin(lc, sub, t.received, errs, t.end):
				}
			}
		}(c)
	}

	select {
	case err := <-fatal:
		t.End()
		wg.Wait()
		return err
	case <-t.end:
		wg.Wait()
		return nil
	}
}

func (t *to) End() {
	close(t.received)
	close(t.end)
}

func (t *to) dedup() {
	for pvr := range t.received {
		seen := t.recent.Add(pvr)
		if !seen {
			t.new <- pvr.pub
		}
	}
	select {
	case <- t.new:   // already closed
	default:
		close(t.new)
	}
}

type subscriptionBeginner interface {
	// begin begins a subscription and writes publications to received and errors to errs
	begin(lc api.Subscriber, sub *api.Subscription, received chan *pubValueReceipt,
		errs chan error, end chan struct{}) error
}

type subscriptionBeginnerImpl struct {
	clientID ecid.ID
	signer client.Signer
	params *ToParameters
}

func (sb *subscriptionBeginnerImpl) begin(
	lc api.Subscriber,
	sub *api.Subscription,
	received chan *pubValueReceipt,
	errs chan error,
	end chan struct{},
) error {

	rq := client.NewSubscribeRequest(sb.clientID, sub)
	ctx, cancel, err := client.NewSignedTimeoutContext(sb.signer, rq, sb.params.Timeout)
	if err != nil {
		return err
	}
	subscribeClient, err := lc.Subscribe(ctx, rq)
	cancel()
	if err != nil {
		return err
	}
	for {
		rp, err := subscribeClient.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		pvr, err := newPublicationValueReceipt(rp.Key, rp.Value, rp.Metadata.PubKey)
		if err != nil {
			return err
		}
		select {
		case <-end:
			return nil
		case received <- pvr:
			errs <- nil
		}
	}
}

func monitorRunningErrorCount(errs chan error, fatal chan error, maxRunningErrRate float32) {
	maxRunningErrCount := int(maxRunningErrRate * errQueueSize)

	// fill error queue with non-errors
	runningErrs := make(chan error, errQueueSize)
	for c := 0; c < errQueueSize; c++ {
		runningErrs <- nil
	}

	// consume from errs and keep running error count; send fatal error if ever above threshold
	runningNErrs := 0
	for latestErr := range errs {
		if latestErr != nil {
			runningNErrs++
			if runningNErrs >= maxRunningErrCount {
				fatal <- ErrTooManySubscriptionErrs
				return
			}
		}
		if earliestErr := <-runningErrs; earliestErr != nil {
			runningNErrs--
		}
		runningErrs <- latestErr
	}
}
