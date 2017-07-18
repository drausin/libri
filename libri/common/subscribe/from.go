package subscribe

import (
	"errors"
	"math/rand"
	"sync"

	"go.uber.org/zap"
)

const (
	// DefaultNMaxSubscriptions is the default maximum number of subscriptions from clients
	// to support.
	DefaultNMaxSubscriptions = 64

	// DefaultEndSubscriptionProb is the default Bernoulli probability of ending a particular
	// subscription.
	DefaultEndSubscriptionProb = 1.0 / (1 << 20)

	fanSlack = 8
)

// ErrNotAcceptingNewSubscriptions indicates when new subscriptions are not being accepted.
var ErrNotAcceptingNewSubscriptions = errors.New("not accepting new subscriptions")

// FromParameters define how the collection of subscriptions from other peer will be managed.
type FromParameters struct {
	// NSubscriptions is the maximum number of concurrent subscriptions from other peers.
	NMaxSubscriptions uint32

	// EndSubscriptionprob is the Bernoulli probability of ending a particular subscription.
	EndSubscriptionProb float64
}

// NewDefaultFromParameters returns a *FromParameters object with default values.
func NewDefaultFromParameters() *FromParameters {
	return &FromParameters{
		NMaxSubscriptions:   DefaultNMaxSubscriptions,
		EndSubscriptionProb: DefaultEndSubscriptionProb,
	}
}

// From manages the fan-out from a main outbound channel to those of all the subscribers.
type From interface {
	// Fanout copies messages from the outbound publication channel to the individual channels
	// of each subscriber.
	Fanout()

	// New creates a new subscriber channel, adds it to the fan-out, and returns it. If
	New() (chan *KeyedPub, chan struct{}, error)
}

type from struct {
	params       *FromParameters
	logger       *zap.Logger
	out          chan *KeyedPub
	fanout       map[uint64]chan *KeyedPub
	done         map[uint64]chan struct{}
	nextFanIndex uint64
	ender        ender
	mu           sync.Mutex
}

// NewFrom creates a new From instance that fans out from the given output channel.
func NewFrom(params *FromParameters, logger *zap.Logger, out chan *KeyedPub) From {
	return &from{
		params: params,
		logger: logger,
		out:    out,
		fanout: make(map[uint64]chan *KeyedPub),
		done:   make(map[uint64]chan struct{}),
		ender: &bernoulliEnder{
			p:   params.EndSubscriptionProb,
			rng: rand.New(rand.NewSource(0)),
		},
	}
}

func (f *from) Fanout() {
	for pub := range f.out {
		for i := range f.fanout {
			if f.ender.end() {
				f.endSubscription(i)
				continue
			}
			select {
			// TODO (drausin) add timeout here for channel accepting value (?)
			case <-f.done[i]:
				f.endSubscription(i)
			case f.fanout[i] <- pub:
			}
		}
		f.removeEndedSubscriptions()
	}
	// end all fanout channels
	for i := range f.fanout {
		f.endSubscription(i)
	}
	f.removeEndedSubscriptions()
}

func (f *from) New() (chan *KeyedPub, chan struct{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if uint32(len(f.fanout)) == f.params.NMaxSubscriptions {
		return nil, nil, ErrNotAcceptingNewSubscriptions
	}
	out := make(chan *KeyedPub, fanSlack)
	done := make(chan struct{})
	f.fanout[f.nextFanIndex] = out
	f.done[f.nextFanIndex] = done
	f.nextFanIndex++
	return out, done, nil
}

func (f *from) endSubscription(i uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.done[i]:
	default:
		// close done[i] if it's not already closed
		close(f.done[i])
	}
	close(f.fanout[i])
	delete(f.fanout, i)
	delete(f.done, i)
}

func (f *from) removeEndedSubscriptions() {
	f.mu.Lock()
	defer f.mu.Unlock()
	toRemove := make([]uint64, 0)
	for i := range f.fanout {
		if _, in := f.done[i]; !in {
			toRemove = append(toRemove, i)
		}
	}
	for _, i := range toRemove {
		delete(f.fanout, i)
	}
}

// ender decides whether to end a subscription to a client.
type ender interface {
	// end decides whether to end subscription i from a client.
	end() bool
}

type bernoulliEnder struct {
	p   float64
	rng *rand.Rand
}

// End ends a subscription with probability p, which is equivalent to sampling a Bernoulli random
// variable p(x = 1 | p). i isn't actually used in this implementation.
func (e *bernoulliEnder) end() bool {
	return e.rng.Float64() < e.p
}
