// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	unitsVerified           prometheus.Counter
	unitsAccepted           prometheus.Counter
	txsSubmitted            prometheus.Counter // includes gossip
	txsVerified             prometheus.Counter
	txsAccepted             prometheus.Counter
	decisionsRPCConnections prometheus.Gauge
	blocksRPCConnections    prometheus.Gauge
}

func newMetrics(gatherer ametrics.MultiGatherer) (*Metrics, error) {
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
	}
	r := prometheus.NewRegistry()
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.unitsVerified),
		r.Register(m.unitsAccepted),
		r.Register(m.txsSubmitted),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
		r.Register(m.decisionsRPCConnections),
		r.Register(m.blocksRPCConnections),
		gatherer.Register("hyper_sdk", r),
	)
	return m, errs.Err
}
