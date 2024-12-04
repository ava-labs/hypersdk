// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "gossiper"

type metrics struct {
	txsReceived            prometheus.Counter
	seenTxsReceived        prometheus.Counter
	txsGossiped            prometheus.Counter
	selectTxsToGossipCount prometheus.Counter
	selectTxsToGossipSum   prometheus.Gauge
	selectedTxsToGossip    prometheus.Counter
	targetTxsSum           prometheus.Gauge
}

func newMetrics(r prometheus.Registerer) (*metrics, error) {
	m := &metrics{
		txsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_received",
			Help:      "number of txs received over gossip",
		}),
		txsGossiped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_gossiped",
			Help:      "number of txs gossiped",
		}),
		seenTxsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "seen_txs_received",
			Help:      "number of txs received over gossip that were already seen",
		}),
		selectTxsToGossipCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "select_txs_to_gossip_count",
			Help:      "number of times gossiper iterates mempool to select txs for gossip",
		}),
		selectTxsToGossipSum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "select_txs_to_gossip_sum",
			Help:      "sum of time spent iterating mempool to select txs for gossip",
		}),
		selectedTxsToGossip: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "selected_txs_to_gossip",
			Help:      "number of txs selected for gossip",
		}),
		targetTxsSum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "target_txs_sum",
			Help:      "sum of time spent selecting target for txs gossip",
		}),
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsReceived),
		r.Register(m.txsGossiped),
		r.Register(m.seenTxsReceived),
		r.Register(m.selectTxsToGossipCount),
		r.Register(m.selectTxsToGossipSum),
		r.Register(m.selectedTxsToGossip),
		r.Register(m.targetTxsSum),
	)
	return m, errs.Err
}
