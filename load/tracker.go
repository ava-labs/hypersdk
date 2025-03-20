// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "load"

var _ Tracker = (*PrometheusTracker)(nil)

type PrometheusTracker struct {
	sync.RWMutex
	outstandingTxs map[ids.ID]time.Time

	// metrics
	txsIssued    prometheus.Counter
	txsConfirmed prometheus.Counter
	txsFailed    prometheus.Counter
	txLatency    prometheus.Summary

	txIssuedCounter    uint64
	txConfirmedCounter uint64
	txFailedCounter    uint64
}

func NewPrometheusTracker(registry *prometheus.Registry) *PrometheusTracker {
	tracker := &PrometheusTracker{
		outstandingTxs: make(map[ids.ID]time.Time),
		txsIssued: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_issued",
			Help:      "Number of transactions issued",
		}),
		txsConfirmed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_confirmed",
			Help:      "Number of transactions confirmed",
		}),
		txsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_failed",
			Help:      "Number of transactions failed",
		}),
		txLatency: prometheus.NewSummary(prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       "tx_latency",
			Help:       "Latency of transactions in milliseconds",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}),
	}

	registry.MustRegister(tracker.txsIssued, tracker.txsConfirmed, tracker.txsFailed, tracker.txLatency)

	return tracker
}

func (t *PrometheusTracker) Issue(id ids.ID, time time.Time) {
	t.Lock()
	t.outstandingTxs[id] = time
	t.txsIssued.Inc()
	t.txIssuedCounter++
	t.Unlock()
}

func (t *PrometheusTracker) ObserveConfirmed(id ids.ID) {
	t.Lock()
	issued := t.outstandingTxs[id]
	delete(t.outstandingTxs, id)
	elapsed := time.Since(issued)

	t.txsConfirmed.Inc()
	t.txLatency.Observe(float64(elapsed.Milliseconds()))
	t.txConfirmedCounter++
	t.Unlock()
}

func (t *PrometheusTracker) ObserveFailed(id ids.ID) {
	t.Lock()
	delete(t.outstandingTxs, id)

	t.txsFailed.Inc()
	t.txFailedCounter++
	t.Unlock()
}

func (t *PrometheusTracker) GetObservedConfirmed() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txConfirmedCounter
}

func (t *PrometheusTracker) GetObservedFailed() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txFailedCounter
}

func (t *PrometheusTracker) GetObservedIssued() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txIssuedCounter
}
