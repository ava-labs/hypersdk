package rchannel

import (
	"context"

	"github.com/ava-labs/hypersdk/smap"
)

type RChannel[V any] struct {
	keys    *smap.SMap[V]
	skips   int
	pending chan string
	done    chan struct{}
	err     error
}

func New[V any](backlog int) *RChannel[V] {
	return &RChannel[V]{
		pending: make(chan string, backlog),
		keys:    smap.New[V](backlog),
		done:    make(chan struct{}),
	}
}

func (r *RChannel[V]) SetCallback(c func(context.Context, string, V) error) {
	go func() {
		defer close(r.done)

		for k := range r.pending {
			v, ok := r.keys.GetAndDelete(k)
			if !ok {
				// Already handled
				r.skips++
				continue
			}

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
	r.keys.Put(key, val)
	r.pending <- key
}

func (r *RChannel[V]) Wait() (int, error) {
	close(r.pending)
	<-r.done
	return r.skips, r.err
}
