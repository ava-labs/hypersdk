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
	if em == nil {
		return
	}
	em.blocked.Inc()
}

func (em *executorMetrics) RecordExecutable() {
	if em == nil {
		return
	}
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
	blockBuildCertsDropped    prometheus.Counter
	stateChanges              prometheus.Counter
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
	mempoolExpired            prometheus.Counter
	fetchChunkAttempts        prometheus.Counter
	uselessFetchChunkAttempts prometheus.Counter
	txGossipDropped           prometheus.Counter
	unitsExecutedBandwidth    prometheus.Counter
	unitsExecutedCompute      prometheus.Counter
	unitsExecutedRead         prometheus.Counter
	unitsExecutedAllocate     prometheus.Counter
	unitsExecutedWrite        prometheus.Counter
	uselessChunkAuth          prometheus.Counter
	optimisticCertifiedGossip prometheus.Counter
	engineBacklog             prometheus.Gauge
	rpcTxBacklog              prometheus.Gauge
	chainDataSize             prometheus.Gauge
	executedProcessingBacklog prometheus.Gauge
	mempoolLen                prometheus.Gauge
	mempoolSize               prometheus.Gauge
	gossipTxBacklog           prometheus.Gauge
	websocketConnections      prometheus.Gauge
	lastAcceptedEpoch         prometheus.Gauge
	lastExecutedEpoch         prometheus.Gauge
	waitRepeat                metric.Averager
	waitQueue                 metric.Averager
	waitAuth                  metric.Averager
	waitExec                  metric.Averager
	waitPrecheck              metric.Averager
	waitCommit                metric.Averager
	waitRoot                  metric.Averager
	chunkBuild                metric.Averager
	blockBuild                metric.Averager
	blockParse                metric.Averager
	blockVerify               metric.Averager
	blockAccept               metric.Averager
	blockProcess              metric.Averager
	blockExecute              metric.Averager
	executedChunkProcess      metric.Averager
	executedBlockProcess      metric.Averager
	fetchMissingChunks        metric.Averager
	collectChunkSignatures    metric.Averager
	txTimeRemainingMempool    metric.Averager
	chunkAuth                 metric.Averager

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
	waitQueue, err := metric.NewAverager(
		"chain",
		"wait_queue",
		"time spent iterating over chunk to queue txs for execution",
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
	waitPrecheck, err := metric.NewAverager(
		"chain",
		"wait_precheck",
		"time spent waiting for precheck",
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
	waitRoot, err := metric.NewAverager(
		"chain",
		"wait_root",
		"time spent waiting to generate root after execution",
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
	executedChunkProcess, err := metric.NewAverager(
		"chain",
		"executed_chunk_process",
		"time spent processing executed chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	executedBlockProcess, err := metric.NewAverager(
		"chain",
		"executed_block_process",
		"time spent processing executed blocks",
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
	collectChunkSignatures, err := metric.NewAverager(
		"chain",
		"collect_chunk_signatures",
		"time spent collecting chunk signatures",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	txTimeRemainingMempool, err := metric.NewAverager(
		"chain",
		"tx_time_remaining_mempool",
		"valid time for inclusion when a tx is included in the mempool",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	chunkAuth, err := metric.NewAverager(
		"chain",
		"chunk_auth",
		"time spent authenticating chunks",
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
		blockBuildCertsDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "block_build_certs_dropped",
			Help:      "number of certs dropped while building blocks",
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
		mempoolExpired: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "mempool_expired",
			Help:      "number of transactions that expired while in the mempool",
		}),
		fetchChunkAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "fetch_chunk_attempts",
			Help:      "number of attempts to fetch a chunk",
		}),
		uselessFetchChunkAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "useless_fetch_chunk_attempts",
			Help:      "number of attempts to fetch a chunk that were useless (received via push)",
		}),
		txGossipDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "tx_gossip_dropped",
			Help:      "number of tx gossip messages dropped",
		}),
		unitsExecutedBandwidth: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_executed_bandwidth",
			Help:      "number of bandwidth units executed",
		}),
		unitsExecutedCompute: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_executed_compute",
			Help:      "number of compute units executed",
		}),
		unitsExecutedRead: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_executed_read",
			Help:      "number of read units executed",
		}),
		unitsExecutedAllocate: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_executed_allocate",
			Help:      "number of allocate units executed",
		}),
		unitsExecutedWrite: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_executed_write",
			Help:      "number of write units executed",
		}),
		optimisticCertifiedGossip: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "optimistic_certified_gossip",
			Help:      "number of optimistic certified messages sent over gossip",
		}),
		uselessChunkAuth: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "useless_chunk_auth",
			Help:      "number of chunks that were authenticated but not executed",
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
		websocketConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "websocket_connections",
			Help:      "number of websocket connections",
		}),
		lastAcceptedEpoch: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "last_accepted_epoch",
			Help:      "last accepted epoch",
		}),
		lastExecutedEpoch: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "last_executed_epoch",
			Help:      "last executed epoch",
		}),
		waitRepeat:             waitRepeat,
		waitQueue:              waitQueue,
		waitAuth:               waitAuth,
		waitExec:               waitExec,
		waitPrecheck:           waitPrecheck,
		waitCommit:             waitCommit,
		waitRoot:               waitRoot,
		chunkBuild:             chunkBuild,
		blockBuild:             blockBuild,
		blockParse:             blockParse,
		blockVerify:            blockVerify,
		blockAccept:            blockAccept,
		blockProcess:           blockProcess,
		blockExecute:           blockExecute,
		executedChunkProcess:   executedChunkProcess,
		executedBlockProcess:   executedBlockProcess,
		fetchMissingChunks:     fetchMissingChunks,
		collectChunkSignatures: collectChunkSignatures,
		txTimeRemainingMempool: txTimeRemainingMempool,
		chunkAuth:              chunkAuth,
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
		r.Register(m.blockBuildCertsDropped),
		r.Register(m.stateChanges),
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
		r.Register(m.mempoolExpired),
		r.Register(m.fetchChunkAttempts),
		r.Register(m.uselessFetchChunkAttempts),
		r.Register(m.engineBacklog),
		r.Register(m.rpcTxBacklog),
		r.Register(m.chainDataSize),
		r.Register(m.executedProcessingBacklog),
		r.Register(m.mempoolLen),
		r.Register(m.mempoolSize),
		r.Register(m.gossipTxBacklog),
		r.Register(m.websocketConnections),
		r.Register(m.lastAcceptedEpoch),
		r.Register(m.lastExecutedEpoch),
		r.Register(m.txGossipDropped),
		r.Register(m.unitsExecutedBandwidth),
		r.Register(m.unitsExecutedCompute),
		r.Register(m.unitsExecutedRead),
		r.Register(m.unitsExecutedAllocate),
		r.Register(m.unitsExecutedWrite),
		r.Register(m.uselessChunkAuth),
		r.Register(m.optimisticCertifiedGossip),
	)
	return r, m, errs.Err
}
