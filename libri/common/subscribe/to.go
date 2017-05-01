package subscribe

import (
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/pkg/errors"
	"io"
	"math/rand"
	"time"
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

	// DefaultNSubscriptions is the default number of subscriptions to other peers to maintain.
	DefaultNSubscriptions = 10

	// DefaultFPRate is the default false positive rate for each subscription to another peer.
	DefaultFPRate = 0.75

	// DefaultTimeout is the default timeout for Subscribe requests.
	DefaultTimeout = 5 * time.Second

	// DefaultMaxErrRate is the maximum allowed error rate for Subscribe requests and received
	// publications before a fatal error is thrown.
	DefaultMaxErrRate = 0.1

	// errQueueSize is the size of the error queue used to calculate the running error rate.
	errQueueSize = 100
)

// Parameters define how the collection of subscriptions will be managed.
type Parameters struct {
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
}

// NewDefaultParameters returns a *Parameters object with default values.
func NewDefaultParameters() *Parameters {
	return &Parameters{
		NSubscriptions: DefaultNSubscriptions,
		FPRate: DefaultFPRate,
		Timeout: DefaultTimeout,
		MaxErrRate: DefaultMaxErrRate,
	}
}

// To maintains active subscriptions to a collection of peers, merging their publications into a
// single, deduplicated stream.
type To interface {
	// Begin starts and runs the subscriptions to the peers. It runs indefinitely until either
	// a fatal error is encountered, or the subscriptions are gracefully stopped via End().
	Begin() error

	// End gracefully stops the active subscriptions.
	End()
}

type to struct {
	params   *Parameters
	cb       api.ClientBalancer
	sb subscriptionBeginner
	recent   RecentPublications
	new      chan *KeyedPub
	end      chan struct{}
}

// NewTo creates a new To instance, writing merged, deduplicated publications to the given new
// channel.
func NewTo(
	params *Parameters,
	clientID ecid.ID,
	cb api.ClientBalancer,
	signer client.Signer,
	recent RecentPublications,
	new chan *KeyedPub,
	end chan struct{},
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
		new: new,
		end: end,
	}
}

func (t *to) Begin() error {
	channelsSlack := t.params.NSubscriptions
	received := make(chan *pubValueReceipt, channelsSlack) // all received pubs
	errs := make(chan error, channelsSlack)                // non-fatal errs and nils
	fatal := make(chan error)                              // signals fatal end

	// dedup all received publications, writing new to t.new & t.recent
	go t.dedup(received)

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
				}
				sub, err := NewFPSubscription(fp, rng)
				if err != nil {
					fatal <- err
				}
				select {
				case <-t.end:
					return
				case errs <- t.sb.begin(lc, sub, received, errs, t.end):
				}
			}
		}(c)
	}

	select {
	case err := <-fatal:
		close(t.end)
		return err
	case <-t.end:
		return nil
	}
}

func (t *to) End() {
	close(t.end)
}

func (t *to) dedup(received chan *pubValueReceipt) {
	for pvr := range received {
		seen := t.recent.Add(pvr)
		if !seen {
			t.new <- pvr.pub
		}
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
	params *Parameters
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
