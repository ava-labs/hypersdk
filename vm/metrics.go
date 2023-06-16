// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	unitsAccepted                 prometheus.Counter
	txsSubmitted                  prometheus.Counter // includes gossip
	txsVerified                   prometheus.Counter
	txBlocksVerified              prometheus.Counter
	txBlocksVerifiedFailed        prometheus.Counter
	txBlocksVerifiedFailedManager prometheus.Counter
	txsAccepted                   prometheus.Counter
	txBlocksAccepted              prometheus.Counter
	stateChanges                  prometheus.Counter
	stateOperations               prometheus.Counter
	txBlocksMissing               prometheus.Counter
	txBlocksDropped               prometheus.Counter
	deletedTxBlocks               prometheus.Counter
	earlyBuildStop                prometheus.Counter
	txBlockBytesSent              prometheus.Counter
	txBlockBytesReceived          prometheus.Counter
	mempoolSize                   prometheus.Gauge
	mempoolSizeAfterBuild         prometheus.Gauge
	acceptorDrift                 prometheus.Gauge
	rootCalculated                metric.Averager
	commitState                   metric.Averager
	waitSignatures                metric.Averager
	buildBlock                    metric.Averager
	verifyWait                    metric.Averager
	txBlockVerify                 metric.Averager
	txBlockIssuanceDiff           metric.Averager
	rootBlockIssuanceDiff         metric.Averager
	rootBlockAcceptanceDiff       metric.Averager
	addVerifyDiff                 metric.Averager
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
	commitState, err := metric.NewAverager(
		"chain",
		"commit_state",
		"time spent committing state in accept",
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
	buildBlock, err := metric.NewAverager(
		"chain",
		"build_block",
		"time spent building block",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	verifyWait, err := metric.NewAverager(
		"chain",
		"verify_wait",
		"time spent waiting for txBlocks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	txBlockVerify, err := metric.NewAverager(
		"chain",
		"tx_block_verify",
		"time spent verifying a tx block",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	txBlockIssuanceDiff, err := metric.NewAverager(
		"chain",
		"tx_block_issuance_diff",
		"delay for tx block to be received after issuance",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	rootBlockIssuanceDiff, err := metric.NewAverager(
		"chain",
		"root_block_issuance_diff",
		"delay for root block to be received after issuance",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	rootBlockAcceptanceDiff, err := metric.NewAverager(
		"chain",
		"root_block_acceptance_diff",
		"delay for root block to be received after issuance",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	addVerifyDiff, err := metric.NewAverager(
		"vm",
		"add_verify_diff",
		"time spent waiting to verify a tx block",
		r,
	)
	if err != nil {
		return nil, nil, err
	}

	m := &Metrics{
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
		txBlocksVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "tx_blocks_verified",
			Help:      "number of tx blocks verified by vm",
		}),
		txBlocksVerifiedFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "tx_blocks_verified_failed",
			Help:      "number of tx blocks that fail verification",
		}),
		txBlocksVerifiedFailedManager: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "tx_blocks_verified_failed_manager",
			Help:      "number of tx blocks that fail verification in manager",
		}),
		txsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_accepted",
			Help:      "number of txs accepted by vm",
		}),
		txBlocksAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "tx_blocks_accepted",
			Help:      "number of tx blocks accepted by vm",
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
		txBlocksMissing: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "tx_blocks_missing",
			Help:      "number of tx blocks missing when root block is parsed",
		}),
		txBlocksDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "tx_blocks_dropped",
			Help:      "number of tx blocks dropped without being verified when root block is accepted",
		}),
		deletedTxBlocks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "deleted_tx_blocks",
			Help:      "number of tx blocks deleted",
		}),
		mempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "number of transactions in the mempool",
		}),
		mempoolSizeAfterBuild: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size_after_build",
			Help:      "number of transactions in the mempool after build",
		}),
		acceptorDrift: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "acceptor_drift",
			Help:      "number of blocks behind tip",
		}),
		earlyBuildStop: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "early_build_stop",
			Help:      "number of blocks we stop building because of time limit",
		}),
		txBlockBytesSent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "tx_block_bytes_sent",
			Help:      "tx block bytes sent",
		}),
		txBlockBytesReceived: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "tx_block_bytes_received",
			Help:      "tx block bytes received",
		}),
		rootCalculated:          rootCalculated,
		commitState:             commitState,
		waitSignatures:          waitSignatures,
		buildBlock:              buildBlock,
		verifyWait:              verifyWait,
		txBlockVerify:           txBlockVerify,
		txBlockIssuanceDiff:     txBlockIssuanceDiff,
		rootBlockIssuanceDiff:   rootBlockIssuanceDiff,
		rootBlockAcceptanceDiff: rootBlockAcceptanceDiff,
		addVerifyDiff:           addVerifyDiff,
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.unitsAccepted),
		r.Register(m.txsSubmitted),
		r.Register(m.txsVerified),
		r.Register(m.txBlocksVerified),
		r.Register(m.txBlocksVerifiedFailed),
		r.Register(m.txBlocksVerifiedFailedManager),
		r.Register(m.txsAccepted),
		r.Register(m.txBlocksAccepted),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.txBlocksMissing),
		r.Register(m.txBlocksDropped),
		r.Register(m.deletedTxBlocks),
		r.Register(m.mempoolSize),
		r.Register(m.mempoolSizeAfterBuild),
		r.Register(m.acceptorDrift),
		r.Register(m.earlyBuildStop),
		r.Register(m.txBlockBytesSent),
		r.Register(m.txBlockBytesReceived),
	)
	return r, m, errs.Err
}
