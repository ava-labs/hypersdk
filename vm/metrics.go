// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	unitsVerified   prometheus.Counter
	unitsAccepted   prometheus.Counter
	txsSubmitted    prometheus.Counter // includes gossip
	txsReceived     prometheus.Counter
	txsGossiped     prometheus.Counter
	txsVerified     prometheus.Counter
	txsAccepted     prometheus.Counter
	stateChanges    prometheus.Counter
	stateOperations prometheus.Counter
	buildCapped     prometheus.Counter
	mempoolSize     prometheus.Gauge
	rootCalculated  metric.Averager
	waitSignatures  metric.Averager
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	r := prometheus.NewRegistry()

	rootCalculated, err := metric.NewAverager(
		"chain",
		"root_calculated",
		"time spent calculating the state root in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitSignatures, err := metric.NewAverager(
		"chain",
		"wait_signatures",
		"time spent waiting for signature verification in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}

	m := &Metrics{
		unitsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_verified",
			Help:      "amount of units verified",
		}),
		unitsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_accepted",
			Help:      "amount of units accepted",
		}),
		txsSubmitted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_submitted",
			Help:      "number of txs submitted to vm",
		}),
		txsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_received",
			Help:      "number of txs received over gossip",
		}),
		txsGossiped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_gossiped",
			Help:      "number of txs gossiped by vm",
		}),
		txsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_verified",
			Help:      "number of txs verified by vm",
		}),
		txsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_accepted",
			Help:      "number of txs accepted by vm",
		}),
		stateChanges: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "state_changes",
			Help:      "number of state changes",
		}),
		stateOperations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "state_operations",
			Help:      "number of state operations",
		}),
		buildCapped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "build_capped",
			Help:      "number of times build capped by target duration",
		}),
		mempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "number of transactions in the mempool",
		}),
		rootCalculated: rootCalculated,
		waitSignatures: waitSignatures,
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.unitsVerified),
		r.Register(m.unitsAccepted),
		r.Register(m.txsSubmitted),
		r.Register(m.txsReceived),
		r.Register(m.txsGossiped),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.mempoolSize),
		r.Register(m.buildCapped),
	)
	return r, m, errs.Err
}
