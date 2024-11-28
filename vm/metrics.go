// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
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
	blockBuild               prometheus.Summary
	blockParse               prometheus.Histogram
	blockVerify              prometheus.Histogram
	blockAccept              prometheus.Histogram
	blockProcess             prometheus.Histogram

	executorBuildRecorder  executor.Metrics
	executorVerifyRecorder executor.Metrics
}

// blockBuckets defines the histogram buckets for block-related metrics.
// The values are in nanoseconds.
var blockBuckets = []float64{
	10_000,        // 10 microseconds
	25_000,        // 25 microseconds
	50_000,        // 50 microseconds
	100_000,       // 100 microseconds
	250_000,       // 250 microseconds
	500_000,       // 500 microseconds
	1_000_000,     // 1 millisecond
	2_500_000,     // 2.5 milliseconds
	5_000_000,     // 5 milliseconds
	10_000_000,    // 10 milliseconds
	25_000_000,    // 25 milliseconds
	50_000_000,    // 50 milliseconds
	100_000_000,   // 100 milliseconds
	250_000_000,   // 250 milliseconds
	500_000_000,   // 500 milliseconds
	1_000_000_000, // 1 second
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	registry := prometheus.NewRegistry()

	blockBuild := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "chain",
			Name:      "block_build",
			Help:      "time spent building blocks",
			Buckets:   blockBuckets,
		},
	)
	blockParse := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "chain",
			Name:      "block_parse",
			Help:      "time spent parsing blocks",
			Buckets:   blockBuckets,
		},
	)
	blockVerify := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "chain",
			Name:      "block_verify",
			Help:      "time spent verifying blocks",
			Buckets:   blockBuckets,
		},
	)
	blockAccept := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "chain",
			Name:      "block_accept",
			Help:      "time spent accepting blocks",
			Buckets:   blockBuckets,
		},
	)
	blockProcess := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "chain",
			Name:      "block_process",
			Help:      "time spent processing blocks",
			Buckets:   blockBuckets,
		},
	)

	metrics := &Metrics{
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
	metrics.executorBuildRecorder = &executorMetrics{blocked: metrics.executorBuildBlocked, executable: metrics.executorBuildExecutable}
	metrics.executorVerifyRecorder = &executorMetrics{blocked: metrics.executorVerifyBlocked, executable: metrics.executorVerifyExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		registry.Register(metrics.txsSubmitted),
		registry.Register(metrics.stateChanges),
		registry.Register(metrics.stateOperations),
		registry.Register(metrics.mempoolSize),
		registry.Register(metrics.buildCapped),
		registry.Register(metrics.emptyBlockBuilt),
		registry.Register(metrics.clearedMempool),
		registry.Register(metrics.deletedBlocks),
		registry.Register(metrics.blocksFromDisk),
		registry.Register(metrics.blocksHeightsFromDisk),
		registry.Register(metrics.executorBuildBlocked),
		registry.Register(metrics.executorBuildExecutable),
		registry.Register(metrics.executorVerifyBlocked),
		registry.Register(metrics.executorVerifyExecutable),
		registry.Register(metrics.bandwidthPrice),
		registry.Register(metrics.computePrice),
		registry.Register(metrics.storageReadPrice),
		registry.Register(metrics.storageAllocatePrice),
		registry.Register(metrics.storageWritePrice),
		registry.Register(metrics.blockBuild),
		registry.Register(metrics.blockParse),
		registry.Register(metrics.blockVerify),
		registry.Register(metrics.blockAccept),
		registry.Register(metrics.blockProcess),
	)
	return registry, metrics, errs.Err
}
