// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/prometheus/client_golang/prometheus"
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
	txsSubmitted          prometheus.Counter // includes gossip
	txsReceived           prometheus.Counter
	txsGossiped           prometheus.Counter
	txsIncluded           prometheus.Counter
	txsValid              prometheus.Counter
	stateChanges          prometheus.Counter
	stateOperations       prometheus.Counter
	clearedMempool        prometheus.Counter
	deletedBlocks         prometheus.Counter
	deletedChunks         prometheus.Counter
	deletedFilteredChunks prometheus.Counter
	blocksFromDisk        prometheus.Counter
	blocksHeightsFromDisk prometheus.Counter
	executorBlocked       prometheus.Counter
	executorExecutable    prometheus.Counter
	chunksReceived        prometheus.Counter
	sigsReceived          prometheus.Counter
	certsReceived         prometheus.Counter
	waitAuth              metric.Averager
	waitCommit            metric.Averager
	waitExec              metric.Averager
	chunkBuild            metric.Averager
	blockBuild            metric.Averager
	blockParse            metric.Averager
	blockVerify           metric.Averager
	blockAccept           metric.Averager
	blockProcess          metric.Averager
	blockExecute          metric.Averager

	executorRecorder executor.Metrics
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	r := prometheus.NewRegistry()

	waitAuth, err := metric.NewAverager(
		"chain",
		"wait_auth",
		"time spent waiting for auth",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitCommit, err := metric.NewAverager(
		"chain",
		"wait_commit",
		"time spent waiting to commit state in execution",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitExec, err := metric.NewAverager(
		"chain",
		"wait_exec",
		"time spent waiting for execution in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	chunkBuild, err := metric.NewAverager(
		"chain",
		"chunk_build",
		"time spent building chunks",
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
	blockExecute, err := metric.NewAverager(
		"chain",
		"block_execute",
		"time spent executing blocks",
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
		txsGossiped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_gossiped",
			Help:      "number of txs gossiped by vm",
		}),
		txsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_received",
			Help:      "number of txs received over gossip",
		}),
		txsIncluded: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_included",
			Help:      "number of txs included in accepted blocks",
		}),
		txsValid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_valid",
			Help:      "number of valid txs included in accepted blocks",
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
		deletedChunks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_chunks",
			Help:      "number of chunks deleted",
		}),
		deletedFilteredChunks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_filtered_chunks",
			Help:      "number of filtered chunks deleted",
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
		executorBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_blocked",
			Help:      "executor tasks blocked during processing",
		}),
		executorExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_executable",
			Help:      "executor tasks executable during processing",
		}),
		chunksReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunks_received",
			Help:      "chunks received from validators",
		}),
		sigsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "sigs_received",
			Help:      "signatures received from validators",
		}),
		certsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "certs_received",
			Help:      "certificates received from validators",
		}),
		waitAuth:     waitAuth,
		waitCommit:   waitCommit,
		waitExec:     waitExec,
		chunkBuild:   chunkBuild,
		blockBuild:   blockBuild,
		blockParse:   blockParse,
		blockVerify:  blockVerify,
		blockAccept:  blockAccept,
		blockProcess: blockProcess,
		blockExecute: blockExecute,
	}
	m.executorRecorder = &executorMetrics{blocked: m.executorBlocked, executable: m.executorExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsSubmitted),
		r.Register(m.txsReceived),
		r.Register(m.txsGossiped),
		r.Register(m.txsIncluded),
		r.Register(m.txsValid),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.clearedMempool),
		r.Register(m.deletedBlocks),
		r.Register(m.deletedChunks),
		r.Register(m.deletedFilteredChunks),
		r.Register(m.blocksFromDisk),
		r.Register(m.blocksHeightsFromDisk),
		r.Register(m.executorBlocked),
		r.Register(m.executorExecutable),
		r.Register(m.chunksReceived),
		r.Register(m.sigsReceived),
		r.Register(m.certsReceived),
	)
	return r, m, errs.Err
}
