package subscribe

import (
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	// DefaultNSubscriptionsTo is the default number of subscriptions to other peers to
	// maintain.
	DefaultNSubscriptionsTo = 10

	// DefaultFPRate is the default false positive rate for each subscription to another peer.
	DefaultFPRate = 0.75

	// DefaultTimeout is the default timeout for Subscribe requests. This is longer than a few
	// seconds because Subscribe responses are dependant on active publications, which may not
	// always be happening.
	DefaultTimeout = 30 * time.Minute

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
	FPRate float32

	// Timeout is the timeout for each Subscribe request.
	Timeout time.Duration

	// MaxErrRate is the maximum allowed error rate for Subscribe requests and received
	// publications before a fatal error is thrown. This value is a running rate over a constant
	// history of responses (c.f., errQueueSize).
	MaxErrRate float32

	// RecentCacheSize is the size of the LRU cache used in deduplicating and grouping
	// publications.
	RecentCacheSize uint32
}

// NewDefaultToParameters returns a *ToParameters object with default values.
func NewDefaultToParameters() *ToParameters {
	return &ToParameters{
		NSubscriptions:  DefaultNSubscriptionsTo,
		FPRate:          DefaultFPRate,
		Timeout:         DefaultTimeout,
		MaxErrRate:      DefaultMaxErrRate,
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

	// Send sends a publication to the channel of received publications.
	Send(pub *api.Publication) error
}

type to struct {
	params   *ToParameters
	logger   *zap.Logger
	clientID ecid.ID
	csb      api.ClientSetBalancer
	sb       subscriptionBeginner
	recent   RecentPublications
	received chan *pubValueReceipt
	new      chan *KeyedPub
	end      chan struct{}
}

// NewTo creates a new To instance, writing merged, deduplicated publications to the given new
// channel.
func NewTo(
	params *ToParameters,
	logger *zap.Logger,
	clientID ecid.ID,
	csb api.ClientSetBalancer,
	signer client.Signer,
	recent RecentPublications,
	new chan *KeyedPub,
) To {
	return &to{
		params:   params,
		logger:   logger,
		csb:      csb,
		clientID: clientID,
		sb: &subscriptionBeginnerImpl{
			clientID: clientID,
			signer:   signer,
			params:   params,
		},
		recent:   recent,
		received: make(chan *pubValueReceipt, params.NSubscriptions),
		new:      new,
		end:      make(chan struct{}),
	}
}

func (t *to) Begin() error {
	channelsSlack := t.params.NSubscriptions
	errs := make(chan error, channelsSlack) // non-fatal errs and nils
	fatal := make(chan error)               // signals fatal end

	// dedup all received publications, writing new to t.new & t.recent
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		t.dedup()
	}(wg)

	// monitor non-fatal errors, sending fatal err if too many
	go monitorRunningErrorCount(errs, fatal, t.params.MaxErrRate, t.logger)

	// subscription threads writing to received & errs channels
	for c := uint32(0); c < t.params.NSubscriptions; c++ {
		go func(i uint32) {
			rng, fp := rand.New(rand.NewSource(int64(i))), float64(t.params.FPRate)
			for {
				lc, peerID, err := t.csb.AddNext()
				if err != nil {
					fatal <- err
					return
				}
				sub, err := NewFPSubscription(fp, rng)
				if err != nil {
					fatal <- err
					return
				}
				t.logger.Debug("beginning new subscription",
					zap.Int("index", int(i)),
					zap.Float64("false_positive_rate", fp),
					zap.Stringer("peer_id", peerID),
				)
				select {
				case <-t.end:
					return
				case errs <- t.sb.begin(lc, sub, t.received, errs, t.end):
				}
				if err := t.csb.Remove(peerID); err != nil {
					panic(err)  // should never happen
				}
			}
		}(c)
	}

	select {
	case err := <-fatal:
		t.logger.Error("fatal subscription error", zap.Error(err))
		t.End()
		wg.Wait()
		return err
	case <-t.end:
		wg.Wait()
		return nil
	}
}

func (t *to) End() {
	t.logger.Info("ending subscriptions")
	select {
	case <-t.received:  // already closed
	default:
		close(t.received)
	}
	select {
	case <- t.end:  // already closed
	default:
		close(t.end)
	}
	t.logger.Debug("ended subscriptions")
}

func (t *to) Send(pub *api.Publication) error {
	if pub == nil {
		return nil
	}
	key, err := api.GetKey(pub)
	if err != nil {
		// should never happen
		return err
	}

	t.logger.Info("sending new publication",
		zap.String("publication_key", key.String()),
	)
	t.logger.Debug("publication value", getLoggerValues(pub)...)
	pvr, err := newPublicationValueReceipt(key.Bytes(), pub, ecid.ToPublicKeyBytes(t.clientID))
	if err != nil {
		return err
	}
	select {
	case <-t.end:
		return errors.New("receive channel closed")
	default:
		t.received <- pvr
		return nil
	}
}

func (t *to) dedup() {
	for pvr := range t.received {
		seen := t.recent.Add(pvr)
		if !seen {
			t.logger.Info("received new publication",
				zap.String("publication_key", pvr.pub.Key.String()),
			)
			t.logger.Debug("publication value", getLoggerValues(pvr.pub.Value)...)
			t.new <- pvr.pub
		}
	}
	select {
	case <-t.new: // already closed
	default:
		close(t.new)
	}
}

func getLoggerValues(pub *api.Publication) []zapcore.Field {
	return []zapcore.Field{
		zap.String("entry_key", fmt.Sprintf("%032x", pub.EntryKey)),
		zap.String("envelope_key", fmt.Sprintf("%032x", pub.EnvelopeKey)),
		zap.String("author_public_key", fmt.Sprintf("%065x", pub.AuthorPublicKey)),
		zap.String("reader_public_key", fmt.Sprintf("%065x", pub.ReaderPublicKey)),
	}
}

type subscriptionBeginner interface {
	// begin begins a subscription and writes publications to received and errors to errs
	begin(lc api.Subscriber, sub *api.Subscription, received chan *pubValueReceipt,
		errs chan error, end chan struct{}) error
}

type subscriptionBeginnerImpl struct {
	clientID ecid.ID
	signer   client.Signer
	params   *ToParameters
}

func (sb *subscriptionBeginnerImpl) begin(
	lc api.Subscriber,
	sub *api.Subscription,
	received chan *pubValueReceipt,
	errs chan error,
	end chan struct{},
) error {

	rq := client.NewSubscribeRequest(sb.clientID, sub)
	ctx, err := client.NewSignedContext(sb.signer, rq)
	if err != nil {
		return err
	}
	subscribeClient, err := lc.Subscribe(ctx, rq)
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
		if rp == nil {
			// receiving channel has already closed
			return nil
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

func monitorRunningErrorCount(
	errs chan error, fatal chan error, maxRunningErrRate float32, logger *zap.Logger,
) {
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
			logger.Debug("received non-fatal subscribeTo error", zap.Error(latestErr))
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
