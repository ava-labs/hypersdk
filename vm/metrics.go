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

// TODO: rename all of these
type Metrics struct {
	txsSubmitted              prometheus.Counter // includes gossip
	txsReceived               prometheus.Counter
	txsGossiped               prometheus.Counter
	txsIncluded               prometheus.Counter
	txsInvalid                prometheus.Counter
	chunkBuildTxsDropped      prometheus.Counter
	stateChanges              prometheus.Counter
	stateOperations           prometheus.Counter
	remainingMempool          prometheus.Counter
	chunkBytesBuilt           prometheus.Counter
	deletedBlocks             prometheus.Counter
	deletedUselessChunks      prometheus.Counter
	deletedIncludedChunks     prometheus.Counter
	deletedFilteredChunks     prometheus.Counter
	blocksFromDisk            prometheus.Counter
	blocksHeightsFromDisk     prometheus.Counter
	executorBlocked           prometheus.Counter
	executorExecutable        prometheus.Counter
	chunksReceived            prometheus.Counter
	sigsReceived              prometheus.Counter
	certsReceived             prometheus.Counter
	chunksExecuted            prometheus.Counter
	chunksNotAuthorized       prometheus.Counter
	unusedChunkAuthorizations prometheus.Counter
	txRPCAuthorized           prometheus.Counter
	blockVerifyFailed         prometheus.Counter
	gossipTxMsgInvalid        prometheus.Counter
	gossipTxInvalid           prometheus.Counter
	gossipChunkInvalid        prometheus.Counter
	gossipChunkSigInvalid     prometheus.Counter
	gossipCertInvalid         prometheus.Counter
	rpcTxInvalid              prometheus.Counter
	expiredBuiltChunks        prometheus.Counter
	expiredCerts              prometheus.Counter
	engineBacklog             prometheus.Gauge
	rpcTxBacklog              prometheus.Gauge
	chainDataSize             prometheus.Gauge
	executedProcessingBacklog prometheus.Gauge
	mempoolLen                prometheus.Gauge
	mempoolSize               prometheus.Gauge
	gossipTxBacklog           prometheus.Gauge
	waitRepeat                metric.Averager
	waitAuth                  metric.Averager
	waitExec                  metric.Averager
	waitProcessor             metric.Averager
	waitCommit                metric.Averager
	chunkBuild                metric.Averager
	blockBuild                metric.Averager
	blockParse                metric.Averager
	blockVerify               metric.Averager
	blockAccept               metric.Averager
	blockProcess              metric.Averager
	blockExecute              metric.Averager
	chunkProcess              metric.Averager
	optimisticChunkAuthorized metric.Averager
	fetchMissingChunks        metric.Averager
	rpcReadDelay              metric.Averager

	executorRecorder executor.Metrics
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	r := prometheus.NewRegistry()

	waitRepeat, err := metric.NewAverager(
		"chain",
		"wait_repeat",
		"time spent waiting for repeat",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitAuth, err := metric.NewAverager(
		"chain",
		"wait_auth",
		"time spent waiting for auth",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitExec, err := metric.NewAverager(
		"chain",
		"wait_exec",
		"time spent waiting for execution after auth finishes",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitProcessor, err := metric.NewAverager(
		"chain",
		"wait_processor",
		"time spent waiting for processor (auth + exec)",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitCommit, err := metric.NewAverager(
		"chain",
		"wait_commit",
		"time spent waiting to commit state after execution",
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
		"time spent processing accepted blocks",
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
	chunkProcess, err := metric.NewAverager(
		"chain",
		"chunk_process",
		"time spent processing executed chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	optimisticChunkAuthorized, err := metric.NewAverager(
		"chain",
		"optimistic_chunk_authorized",
		"time spent optimistically verifying chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	fetchMissingChunks, err := metric.NewAverager(
		"chain",
		"fetch_missing_chunks",
		"time spent fetching missing chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	rpcReadDelay, err := metric.NewAverager(
		"chain",
		"rpc_read_delay",
		"delay to read from client (noisy)",
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
		chunkBuildTxsDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunk_build_txs_dropped",
			Help:      "number of txs dropped while building chunks",
		}),
		txsInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_invalid",
			Help:      "number of invalid txs included in accepted blocks",
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
		remainingMempool: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "remaining_mempool",
			Help:      "number of times mempool not cleared while building",
		}),
		chunkBytesBuilt: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunk_bytes_built",
			Help:      "number of bytes in built chunks",
		}),
		deletedBlocks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_blocks",
			Help:      "number of blocks deleted",
		}),
		deletedUselessChunks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_useless_chunks",
			Help:      "number of useless chunks deleted",
		}),
		deletedIncludedChunks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_included_chunks",
			Help:      "number of included chunks deleted",
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
		chunksExecuted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunks_executed",
			Help:      "chunks executed by the engine",
		}),
		chunksNotAuthorized: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunks_not_authorized",
			Help:      "chunks with signatures not verified by the time they are executed",
		}),
		unusedChunkAuthorizations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "unused_chunk_authorizations",
			Help:      "chunks verified but not executed",
		}),
		txRPCAuthorized: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "tx_rpc_authorized",
			Help:      "number of txs authorized during RPC processing",
		}),
		blockVerifyFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "block_verify_failed",
			Help:      "number of blocks that failed verification",
		}),
		gossipTxMsgInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "gossip_tx_msg_invalid",
			Help:      "number of invalid transaction messages received over gossip",
		}),
		gossipTxInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "gossip_tx_invalid",
			Help:      "number of invalid transactions received over gossip",
		}),
		gossipChunkInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "gossip_chunk_invalid",
			Help:      "number of invalid chunks received over gossip",
		}),
		gossipChunkSigInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "gossip_chunk_sig_invalid",
			Help:      "number of invalid chunk signatures received over gossip",
		}),
		gossipCertInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "gossip_cert_invalid",
			Help:      "number of invalid certificates received over gossip",
		}),
		rpcTxInvalid: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "rpc_tx_invalid",
			Help:      "number of invalid transactions received over RPC",
		}),
		expiredBuiltChunks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "expired_built_chunks",
			Help:      "number of chunks that expired after being built",
		}),
		expiredCerts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "expired_certs",
			Help:      "number of certificates that expired",
		}),
		engineBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "engine_backlog",
			Help:      "number of blocks waiting to be executed",
		}),
		rpcTxBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "rpc_tx_backlog",
			Help:      "number of transactions waiting to be processed from RPC",
		}),
		chainDataSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "data_size",
			Help:      "size of the chain data directory",
		}),
		executedProcessingBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "executed_processing_backlog",
			Help:      "number of blocks waiting to be processed after execution",
		}),
		mempoolLen: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_len",
			Help:      "number of transactions in the mempool",
		}),
		mempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "bytes in the mempool",
		}),
		gossipTxBacklog: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "gossip_tx_backlog",
			Help:      "number of transactions waiting to be processed from gossip",
		}),
		waitRepeat:                waitRepeat,
		waitAuth:                  waitAuth,
		waitExec:                  waitExec,
		waitProcessor:             waitProcessor,
		waitCommit:                waitCommit,
		chunkBuild:                chunkBuild,
		blockBuild:                blockBuild,
		blockParse:                blockParse,
		blockVerify:               blockVerify,
		blockAccept:               blockAccept,
		blockProcess:              blockProcess,
		blockExecute:              blockExecute,
		chunkProcess:              chunkProcess,
		optimisticChunkAuthorized: optimisticChunkAuthorized,
		fetchMissingChunks:        fetchMissingChunks,
		rpcReadDelay:              rpcReadDelay,
	}
	m.executorRecorder = &executorMetrics{blocked: m.executorBlocked, executable: m.executorExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.txsSubmitted),
		r.Register(m.txsReceived),
		r.Register(m.txsGossiped),
		r.Register(m.txsIncluded),
		r.Register(m.txsInvalid),
		r.Register(m.chunkBuildTxsDropped),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.remainingMempool),
		r.Register(m.chunkBytesBuilt),
		r.Register(m.deletedBlocks),
		r.Register(m.deletedUselessChunks),
		r.Register(m.deletedIncludedChunks),
		r.Register(m.deletedFilteredChunks),
		r.Register(m.blocksFromDisk),
		r.Register(m.blocksHeightsFromDisk),
		r.Register(m.executorBlocked),
		r.Register(m.executorExecutable),
		r.Register(m.chunksReceived),
		r.Register(m.sigsReceived),
		r.Register(m.certsReceived),
		r.Register(m.chunksExecuted),
		r.Register(m.chunksNotAuthorized),
		r.Register(m.unusedChunkAuthorizations),
		r.Register(m.txRPCAuthorized),
		r.Register(m.blockVerifyFailed),
		r.Register(m.gossipTxMsgInvalid),
		r.Register(m.gossipTxInvalid),
		r.Register(m.gossipChunkInvalid),
		r.Register(m.gossipChunkSigInvalid),
		r.Register(m.gossipCertInvalid),
		r.Register(m.rpcTxInvalid),
		r.Register(m.expiredBuiltChunks),
		r.Register(m.expiredCerts),
		r.Register(m.engineBacklog),
		r.Register(m.rpcTxBacklog),
		r.Register(m.chainDataSize),
		r.Register(m.executedProcessingBacklog),
		r.Register(m.mempoolLen),
		r.Register(m.mempoolSize),
		r.Register(m.gossipTxBacklog),
	)
	return r, m, errs.Err
}
