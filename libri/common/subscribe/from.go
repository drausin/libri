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
	New() chan *api.Publication
}

type from struct {
	out    chan *api.Publication
	fanout map[int]chan *api.Publication
	mu     sync.Mutex
}

func NewFrom(out chan *api.Publication) From {
	return &from{
		out: out,
		fanout: make(map[int]chan *api.Publication),
	}
}

func (f *from) Fanout() {
	for pub := range f.out {
		f.mu.Lock()
		for i := range f.fanout {
			select {
			// TODO (drausin) add timeout here for channel accepting value (?)
			case <-f.fanout[i]:
				// remove closed channel
				delete(f.fanout, i)
			case f.fanout[i] <- pub:
			}

		}
		f.mu.Unlock()
	}
}

func (f *from) New() chan *api.Publication {
	out := make(chan *api.Publication, fanSlack)
	i := len(f.fanout)
	f.mu.Lock()
	f.fanout[i] = out
	f.mu.Unlock()
	return out
}
