// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/internal/executor"
)

type executorMetrics struct {
	blocked    prometheus.Counter
	executable prometheus.Counter
}

func (em *executorMetrics) RecordBlocked() {
	em.blocked.Inc()
}

func (em *executorMetrics) RecordExecutable() {
	em.executable.Inc()
}

type Metrics struct {
	txsSubmitted             prometheus.Counter // includes gossip
	stateChanges             prometheus.Counter
	stateOperations          prometheus.Counter
	buildCapped              prometheus.Counter
	emptyBlockBuilt          prometheus.Counter
	clearedMempool           prometheus.Counter
	deletedBlocks            prometheus.Counter
	blocksFromDisk           prometheus.Counter
	blocksHeightsFromDisk    prometheus.Counter
	executorBuildBlocked     prometheus.Counter
	executorBuildExecutable  prometheus.Counter
	executorVerifyBlocked    prometheus.Counter
	executorVerifyExecutable prometheus.Counter
	mempoolSize              prometheus.Gauge
	bandwidthPrice           prometheus.Gauge
	computePrice             prometheus.Gauge
	storageReadPrice         prometheus.Gauge
	storageAllocatePrice     prometheus.Gauge
	storageWritePrice        prometheus.Gauge
	blockBuild               metric.Averager
	blockParse               metric.Averager
	blockVerify              metric.Averager
	blockAccept              metric.Averager
	blockProcess             metric.Averager

	executorBuildRecorder  executor.Metrics
	executorVerifyRecorder executor.Metrics
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	r := prometheus.NewRegistry()

	blockBuild, err := metric.NewAverager(
		"chain_block_build",
		"time spent building blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockParse, err := metric.NewAverager(
		"chain_block_parse",
		"time spent parsing blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockVerify, err := metric.NewAverager(
		"chain_block_verify",
		"time spent verifying blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockAccept, err := metric.NewAverager(
		"chain_block_accept",
		"time spent accepting blocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	blockProcess, err := metric.NewAverager(
		"chain_block_process",
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
		blocksFromDisk: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "blocks_from_disk",
			Help:      "number of blocks attempted to load from disk",
		}),
		blocksHeightsFromDisk: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "block_heights_from_disk",
			Help:      "number of block heights attempted to load from disk",
		}),
		executorBuildBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_build_blocked",
			Help:      "executor tasks blocked during build",
		}),
		executorBuildExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_build_executable",
			Help:      "executor tasks executable during build",
		}),
		executorVerifyBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_verify_blocked",
			Help:      "executor tasks blocked during verify",
		}),
		executorVerifyExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_verify_executable",
			Help:      "executor tasks executable during verify",
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
		storageAllocatePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_create_price",
			Help:      "unit price of storage creates",
		}),
		storageWritePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_modify_price",
			Help:      "unit price of storage modifications",
		}),
		blockBuild:   blockBuild,
		blockParse:   blockParse,
		blockVerify:  blockVerify,
		blockAccept:  blockAccept,
		blockProcess: blockProcess,
	}
	m.executorBuildRecorder = &executorMetrics{blocked: m.executorBuildBlocked, executable: m.executorBuildExecutable}
	m.executorVerifyRecorder = &executorMetrics{blocked: m.executorVerifyBlocked, executable: m.executorVerifyExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsSubmitted),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.mempoolSize),
		r.Register(m.buildCapped),
		r.Register(m.emptyBlockBuilt),
		r.Register(m.clearedMempool),
		r.Register(m.deletedBlocks),
		r.Register(m.blocksFromDisk),
		r.Register(m.blocksHeightsFromDisk),
		r.Register(m.executorBuildBlocked),
		r.Register(m.executorBuildExecutable),
		r.Register(m.executorVerifyBlocked),
		r.Register(m.executorVerifyExecutable),
		r.Register(m.bandwidthPrice),
		r.Register(m.computePrice),
		r.Register(m.storageReadPrice),
		r.Register(m.storageAllocatePrice),
		r.Register(m.storageWritePrice),
	)
	return r, m, errs.Err
}
