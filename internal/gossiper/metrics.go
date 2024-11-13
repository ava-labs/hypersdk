// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	txsReceived     prometheus.Counter
	seenTxsReceived prometheus.Counter
	txsGossiped     prometheus.Counter
}

func newMetrics(r prometheus.Registerer) (*metrics, error) {
	m := &metrics{
		txsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "gossiper",
			Name:      "txs_received",
			Help:      "number of txs received over gossip",
		}),
		txsGossiped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "gossiper",
			Name:      "txs_gossiped",
			Help:      "number of txs gossiped",
		}),
		seenTxsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "gossiper",
			Name:      "seen_txs_received",
			Help:      "number of txs received over gossip that were already seen",
		}),
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsReceived),
		r.Register(m.txsGossiped),
		r.Register(m.seenTxsReceived),
	)
	return m, errs.Err
}
