package subscribe

import (
	"github.com/drausin/libri/libri/librarian/api"
	"sync"
)

const fanSlack = 8

// From manages the fan-out from a main outbound channel to those of all the subscribers.
type From interface {
	// Fanout copies messages from the outbound publication channel to the individual channels
	// of each subscriber.
	Fanout()

	// New creates a new subscriber channel, adds it to the fan-out, and returns it.
	New() (chan *api.Publication, chan struct{})
}

type from struct {
	out    chan *api.Publication
	fanout map[uint64]chan *api.Publication
	done map[uint64]chan struct{}
	nextFanIndex uint64
	mu     sync.Mutex
}

func NewFrom(out chan *api.Publication) From {
	return &from{
		out: out,
		fanout: make(map[uint64]chan *api.Publication),
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

func (f *from) New() (chan *api.Publication, chan struct{}) {
	out := make(chan *api.Publication, fanSlack)
	done := make(chan struct{})
	f.mu.Lock()
	f.fanout[f.nextFanIndex] = out
	f.done[f.nextFanIndex] = done
	f.nextFanIndex++
	f.mu.Unlock()
	return out, done
}
