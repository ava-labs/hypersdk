package rchannel

import (
	"context"
	"sync"
)

type RChannel[V any] struct {
	l       sync.Mutex
	pending chan string
	keys    map[string]V
	done    chan struct{}
	err     error
}

func New[V any](backlog int) *RChannel[V] {
	return &RChannel[V]{
		pending: make(chan string, backlog),
		keys:    make(map[string]V, backlog),
		done:    make(chan struct{}),
	}
}

func (r *RChannel[V]) SetCallback(c func(context.Context, string, V) error) {
	go func() {
		defer close(r.done)

		for k := range r.pending {
			r.l.Lock()
			v, ok := r.keys[k]
			if !ok {
				// Already handled
				r.l.Unlock()
				continue
			}
			delete(r.keys, k)
			r.l.Unlock()

			// Skip if we already errored (keep dequeing to prevent stall)
			if r.err != nil {
				continue
			}

			// Record error if was unsuccessful
			if err := c(context.TODO(), k, v); err != nil {
				r.err = err
			}
		}
	}()
}

func (r *RChannel[V]) Add(key string, val V) {
	r.l.Lock()
	r.keys[key] = val
	r.l.Unlock()

	r.pending <- key
}

func (r *RChannel[V]) Wait() error {
	close(r.pending)
	<-r.done
	return r.err
}
