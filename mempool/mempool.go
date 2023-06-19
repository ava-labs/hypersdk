// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/prometheus/client_golang/prometheus"
)

const maxPrealloc = 4_096

type Mempool[T Item] struct {
	tracer  trace.Tracer
	metrics *metrics

	c    chan T
	size atomic.Int32
}

// New creates a new [Mempool]. [maxSize] must be > 0 or else the
// implementation may panic.
func New[T Item](
	tracer trace.Tracer,
	maxSize int,
) (*Mempool[T], *prometheus.Registry, error) {
	m := &Mempool[T]{
		tracer: tracer,
		c:      make(chan T, maxSize),
	}
	registry, metrics, err := newMetrics()
	if err != nil {
		return nil, nil, err
	}
	m.metrics = metrics
	return m, registry, nil
}

type metrics struct {
	buildOverhead   metric.Averager
	setMinTimestamp metric.Averager
	add             metric.Averager
	leaseCreate     metric.Averager
}

func newMetrics() (*prometheus.Registry, *metrics, error) {
	r := prometheus.NewRegistry()
	buildOverhead, err := metric.NewAverager(
		"mempool",
		"build_overhead",
		"time spent handling mempool build",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	add, err := metric.NewAverager(
		"mempool",
		"add",
		"time spent adding",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	m := &metrics{
		buildOverhead: buildOverhead,
		add:           add,
		// TODO: add size
	}
	return r, m, nil
}

// Add pushes all new items from [items] to th. Does not add a item if
// the item payer is not exempt and their items in the mempool exceed th.maxPayerSize.
// If the size of th exceeds th.maxSize, Add pops the lowest value item
// from th.pm.
func (th *Mempool[T]) Add(ctx context.Context, items []T) {
	_, span := th.tracer.Start(ctx, "Mempool.Add")
	defer span.End()

	start := time.Now()
	defer func() {
		th.metrics.add.Observe(float64(time.Since(start)))
	}()

	for _, item := range items {
		th.c <- item
		th.size.Add(1)
	}
}

// Len returns the number of items in th.
func (th *Mempool[T]) Len(ctx context.Context) int {
	_, span := th.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	return int(th.size.Load())
}

// TODO: break build apart into:
// * fetch of X txs (kept in pending map to avoid duplicate entry)
//   - could use MaxUnits to determine and try to build 1 tx block at a time
//
// * pre-fetch state, iterate repeatedly until max time has elapsed our out of
// txs
func (th *Mempool[T]) Build(
	ctx context.Context,
	minBuildTime time.Duration,
	maxBuildTime time.Duration,
	f func(context.Context, T) (cont bool, restore bool, err error),
) error {
	ctx, span := th.tracer.Start(ctx, "Mempool.Build")
	defer span.End()

	start := time.Now()
	var vmTime time.Duration
	defer func() {
		th.metrics.buildOverhead.Observe(float64(time.Since(start) - vmTime))
	}()

	restorableItems := []T{}
	var (
		err  error
		stop bool
	)
	for !stop || time.Since(start) > maxBuildTime {
		select {
		case max := <-th.c:
			vmStart := time.Now()
			cont, restore, fErr := f(ctx, max)
			vmTime += time.Since(vmStart)
			if restore {
				// Waiting to restore unused transactions ensures that an account will be
				// excluded from future price mempool iterations
				restorableItems = append(restorableItems, max)
			} else {
				th.size.Add(-1)
			}
			if !cont || fErr != nil {
				err = fErr
				stop = true
				break
			}
		default:
			if time.Since(start) < minBuildTime {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			stop = true
			break
		}
	}
	//
	// Restore unused items
	for _, item := range restorableItems {
		th.c <- item
	}
	return err
}
