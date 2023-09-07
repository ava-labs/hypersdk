// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	txsSubmitted       prometheus.Counter // includes gossip
	txsReceived        prometheus.Counter
	seenTxsReceived    prometheus.Counter
	txsGossiped        prometheus.Counter
	txsVerified        prometheus.Counter
	txsAccepted        prometheus.Counter
	stateChanges       prometheus.Counter
	stateOperations    prometheus.Counter
	buildCapped        prometheus.Counter
	emptyBlockBuilt    prometheus.Counter
	clearedMempool     prometheus.Counter
	deletedBlocks      prometheus.Counter
	mempoolSize        prometheus.Gauge
	bandwidthPrice     prometheus.Gauge
	computePrice       prometheus.Gauge
	storageReadPrice   prometheus.Gauge
	storageCreatePrice prometheus.Gauge
	storageModifyPrice prometheus.Gauge
	rootCalculated     metric.Averager
	waitRoot           metric.Averager
	waitSignatures     metric.Averager
	blockBuild         metric.Averager
	blockParse         metric.Averager
	blockVerify        metric.Averager
	blockAccept        metric.Averager
	blockProcess       metric.Averager
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
	waitRoot, err := metric.NewAverager(
		"chain",
		"wait_root",
		"time spent waiting for root calculation in verify",
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
	blockBuild, err := metric.NewAverager(
		"chain",
		"block_build",
		"time spent building blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockParse, err := metric.NewAverager(
		"chain",
		"block_parse",
		"time spent parsing blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockVerify, err := metric.NewAverager(
		"chain",
		"block_verify",
		"time spent verifying blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockAccept, err := metric.NewAverager(
		"chain",
		"block_accept",
		"time spent accepting blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockProcess, err := metric.NewAverager(
		"chain",
		"block_process",
		"time spent processing blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}

	m := &Metrics{
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
		seenTxsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "seen_txs_received",
			Help:      "number of txs received over gossip that we've already seen",
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
		emptyBlockBuilt: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "empty_block_built",
			Help:      "number of times empty block built",
		}),
		clearedMempool: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "cleared_mempool",
			Help:      "number of times cleared mempool while building",
		}),
		deletedBlocks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_blocks",
			Help:      "number of blocks deleted",
		}),
		mempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "number of transactions in the mempool",
		}),
		bandwidthPrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "bandwidth_price",
			Help:      "unit price of bandwidth",
		}),
		computePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "compute_price",
			Help:      "unit price of compute",
		}),
		storageReadPrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_read_price",
			Help:      "unit price of storage reads",
		}),
		storageCreatePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_create_price",
			Help:      "unit price of storage creates",
		}),
		storageModifyPrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_modify_price",
			Help:      "unit price of storage modifications",
		}),
		rootCalculated: rootCalculated,
		waitRoot:       waitRoot,
		waitSignatures: waitSignatures,
		blockBuild:     blockBuild,
		blockParse:     blockParse,
		blockVerify:    blockVerify,
		blockAccept:    blockAccept,
		blockProcess:   blockProcess,
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsSubmitted),
		r.Register(m.txsReceived),
		r.Register(m.seenTxsReceived),
		r.Register(m.txsGossiped),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.mempoolSize),
		r.Register(m.buildCapped),
		r.Register(m.emptyBlockBuilt),
		r.Register(m.clearedMempool),
		r.Register(m.deletedBlocks),
		r.Register(m.bandwidthPrice),
		r.Register(m.computePrice),
		r.Register(m.storageReadPrice),
		r.Register(m.storageCreatePrice),
		r.Register(m.storageModifyPrice),
	)
	return r, m, errs.Err
}
