package subscribe

import (
	"sync"
	"errors"
)

const (
	// DefaultNMaxSubscriptions is the default maximum number of subscriptions from clients
	// to support.
	DefaultNMaxSubscriptions = 64

	fanSlack = 8
)

// ErrNotAcceptingNewSubscriptions indicates when new subscriptions are not being accepted.
var ErrNotAcceptingNewSubscriptions = errors.New("not accepting new subscriptions")

// FromParameters define how the collection of subscriptions from other peer will be managed.
type FromParameters struct {
	// NSubscriptions is the maximum number of concurrent subscriptions from other peers.
	NMaxSubscriptions uint32
}

// NewDefaultFromParameters returns a *FromParameters object with default values.
func NewDefaultFromParameters() *FromParameters {
	return &FromParameters{
		NMaxSubscriptions: DefaultNMaxSubscriptions,
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
	params *FromParameters
	out    chan *KeyedPub
	fanout map[uint64]chan *KeyedPub
	done map[uint64]chan struct{}
	nextFanIndex uint64
	mu     sync.Mutex
}

// NewFrom creates a new From instance that fans out from the given output channel.
func NewFrom(params *FromParameters, out chan *KeyedPub) From {
	return &from{
		params: params,
		out: out,
		fanout: make(map[uint64]chan *KeyedPub),
		done: make(map[uint64]chan struct{}),
	}
}

func (f *from) Fanout() {
	for pub := range f.out {
		for i := range f.fanout {
			select {
			// TODO (drausin) add timeout here for channel accepting value (?)
			case <- f.done[i]:
				f.mu.Lock()
				close(f.fanout[i])
				delete(f.fanout, i)
				delete(f.done, i)
				f.mu.Unlock()
			case f.fanout[i] <- pub:
			}
		}
	}
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
