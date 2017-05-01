package subscribe

import (
	"sync"
)

const fanSlack = 8

// From manages the fan-out from a main outbound channel to those of all the subscribers.
type From interface {
	// Fanout copies messages from the outbound publication channel to the individual channels
	// of each subscriber.
	Fanout()

	// New creates a new subscriber channel, adds it to the fan-out, and returns it.
	New() (chan *KeyedPub, chan struct{})
}

type from struct {
	out    chan *KeyedPub
	fanout map[uint64]chan *KeyedPub
	done map[uint64]chan struct{}
	nextFanIndex uint64
	mu     sync.Mutex
}

// NewFrom creates a new From instance that fans out from the given output channel.
func NewFrom(out chan *KeyedPub) From {
	return &from{
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

func (f *from) New() (chan *KeyedPub, chan struct{}) {
	out := make(chan *KeyedPub, fanSlack)
	done := make(chan struct{})
	f.mu.Lock()
	f.fanout[f.nextFanIndex] = out
	f.done[f.nextFanIndex] = done
	f.nextFanIndex++
	f.mu.Unlock()
	return out, done
}
