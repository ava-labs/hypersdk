// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	unitsVerified           prometheus.Counter
	unitsAccepted           prometheus.Counter
	txsSubmitted            prometheus.Counter // includes gossip
	txsVerified             prometheus.Counter
	txsAccepted             prometheus.Counter
	stateChanges            prometheus.Counter
	stateOperations         prometheus.Counter
	decisionsRPCConnections prometheus.Gauge
	blocksRPCConnections    prometheus.Gauge
	rootCalculated          metric.Averager
	waitSignatures          metric.Averager
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
		decisionsRPCConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vm",
			Name:      "decisions_rpc_connections",
			Help:      "number of open decisions connections",
		}),
		blocksRPCConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vm",
			Name:      "blocks_rpc_connections",
			Help:      "number of open blocks connections",
		}),
		rootCalculated: rootCalculated,
		waitSignatures: waitSignatures,
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.unitsVerified),
		r.Register(m.unitsAccepted),
		r.Register(m.txsSubmitted),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.decisionsRPCConnections),
		r.Register(m.blocksRPCConnections),
	)
	return r, m, errs.Err
}
