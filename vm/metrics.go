// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	txsAccepted  prometheus.Counter
	txsVerified  prometheus.Counter
	txsSubmitted prometheus.Counter // includes gossip
	mempoolSize  prometheus.Gauge
}

func newMetrics(r prometheus.Registerer) (*Metrics, error) {
	m := &Metrics{
		txsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_accepted",
			Help:      "number of txs accepted",
		}),
		txsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_verified",
			Help:      "number of txs verified",
		}),
		txsSubmitted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_submitted",
			Help:      "number of txs submitted to vm",
		}),
		mempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "number of transactions in the mempool",
		}),
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsVerified),
		r.Register(m.txsSubmitted),
		r.Register(m.mempoolSize),
	)
	return m, errs.Err
}
