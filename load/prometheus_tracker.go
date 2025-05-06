// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "load"

var _ Tracker[any] = (*PrometheusTracker[any])(nil)

type PrometheusTracker[T comparable] struct {
	lock sync.RWMutex

	outstandingTxs map[T]time.Time

	txsIssued    uint64
	txsConfirmed uint64
	txsFailed    uint64

	// metrics
	txsIssuedCounter    prometheus.Counter
	txsConfirmedCounter prometheus.Counter
	txsFailedCounter    prometheus.Counter
	txLatency           prometheus.Histogram
}

type PrometheusRegistry interface {
	prometheus.Registerer
	prometheus.Gatherer
}

func NewPrometheusTracker[T comparable](reg PrometheusRegistry) (*PrometheusTracker[T], error) {
	prometheusTracker := &PrometheusTracker[T]{
		outstandingTxs: make(map[T]time.Time),
		txsIssuedCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_issued",
			Help:      "Number of transactions issued",
		}),
		txsConfirmedCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_confirmed",
			Help:      "Number of transactions confirmed",
		}),
		txsFailedCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_failed",
			Help:      "Number of transactions failed",
		}),
		txLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "tx_latency",
			Help:      "Latency of transactions",
		}),
	}

	errs := wrappers.Errs{}
	errs.Add(
		reg.Register(prometheusTracker.txsIssuedCounter),
		reg.Register(prometheusTracker.txsConfirmedCounter),
		reg.Register(prometheusTracker.txsFailedCounter),
		reg.Register(prometheusTracker.txLatency),
	)
	return prometheusTracker, errs.Err
}

func (p *PrometheusTracker[T]) GetObservedConfirmed() uint64 {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.txsConfirmed
}

func (p *PrometheusTracker[T]) GetObservedFailed() uint64 {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.txsFailed
}

func (p *PrometheusTracker[T]) GetObservedIssued() uint64 {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.txsIssued
}

func (p *PrometheusTracker[T]) Issue(tx T) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.outstandingTxs[tx] = time.Now()
	p.txsIssued++
	p.txsIssuedCounter.Inc()
}

func (p *PrometheusTracker[T]) ObserveConfirmed(tx T) {
	p.lock.Lock()
	defer p.lock.Unlock()

	startTime := p.outstandingTxs[tx]
	delete(p.outstandingTxs, tx)

	p.txsConfirmed++
	p.txsConfirmedCounter.Inc()
	p.txLatency.Observe(float64(time.Since(startTime).Milliseconds()))
}

func (p *PrometheusTracker[T]) ObserveFailed(tx T) {
	p.lock.Lock()
	defer p.lock.Unlock()

	startTime := p.outstandingTxs[tx]
	delete(p.outstandingTxs, tx)

	p.txsFailed++
	p.txsFailedCounter.Inc()
	p.txLatency.Observe(float64(time.Since(startTime).Milliseconds()))
}

// String returns a string representation of the tracker stats.
func (p *PrometheusTracker[T]) String() string {
	p.lock.Lock()
	defer p.lock.Unlock()

	return fmt.Sprintf("Tracker stats: issued=%d, confirmed=%d, failed=%d, inflight=%d",
		p.txsIssued, p.txsConfirmed, p.txsFailed, len(p.outstandingTxs))
}
