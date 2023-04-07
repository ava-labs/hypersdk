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
	chunkRequests           prometheus.Counter
	failedChunkRequests     prometheus.Counter
	chunkJobFails           prometheus.Counter
	txsSubmitted            prometheus.Counter // includes gossip
	txsVerified             prometheus.Counter
	txsAccepted             prometheus.Counter
	decisionsRPCConnections prometheus.Gauge
	blocksRPCConnections    prometheus.Gauge
	chunksFetched           metric.Averager
	rootCalculated          metric.Averager
	waitSignatures          metric.Averager
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	r := prometheus.NewRegistry()

	chunksFetched, err := metric.NewAverager(
		"chain",
		"chunks_fetched",
		"time spent fetching chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
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
		chunkRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunk_requests",
			Help:      "number of chunk requests",
		}),
		failedChunkRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "failed_chunk_requests",
			Help:      "number of failed chunk requests",
		}),
		chunkJobFails: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunk_job_fails",
			Help:      "number of chunk jobs that failed",
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
		chunksFetched:  chunksFetched,
		rootCalculated: rootCalculated,
		waitSignatures: waitSignatures,
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.unitsVerified),
		r.Register(m.unitsAccepted),
		r.Register(m.chunkRequests),
		r.Register(m.failedChunkRequests),
		r.Register(m.chunkJobFails),
		r.Register(m.txsSubmitted),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
		r.Register(m.decisionsRPCConnections),
		r.Register(m.blocksRPCConnections),
	)
	return r, m, errs.Err
}
