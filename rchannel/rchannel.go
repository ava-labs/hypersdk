package rchannel

import (
	"context"
	"sync/atomic"

	"github.com/ava-labs/hypersdk/smap"
)

type RChannel[V any] struct {
	keys    *smap.SMap[V]
	pending chan string
	done    chan struct{}
	err     error

	skips      int
	maxBacklog int32
	backlog    atomic.Int32
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
			r.backlog.Add(-1)
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
	if l := r.backlog.Add(1); l > r.maxBacklog {
		r.maxBacklog = l
	}
	r.keys.Put(key, val)
	r.pending <- key
}

func (r *RChannel[V]) Wait() (int, int, error) {
	close(r.pending)
	<-r.done
	return r.skips, int(r.maxBacklog), r.err
}
