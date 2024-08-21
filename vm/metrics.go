// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/executor"
)

type ExecutorMetrics struct {
	blocked    prometheus.Counter
	executable prometheus.Counter
}

func (em *ExecutorMetrics) RecordBlocked() {
	em.blocked.Inc()
}

func (em *ExecutorMetrics) RecordExecutable() {
	em.executable.Inc()
}

type Metrics struct {
	txsSubmitted             prometheus.Counter // includes gossip
	txsReceived              prometheus.Counter
	seenTxsReceived          prometheus.Counter
	txsGossiped              prometheus.Counter
	txsVerified              prometheus.Counter
	txsAccepted              prometheus.Counter
	stateChanges             prometheus.Counter
	stateOperations          prometheus.Counter
	allocKeyOperations       prometheus.Counter
	writeKeyOperations       prometheus.Counter
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
	rootCalculated           metric.Averager
	waitRoot                 metric.Averager
	waitSignatures           metric.Averager
	blockBuild               metric.Averager
	blockParse               metric.Averager
	blockVerify              metric.Averager
	blockAccept              metric.Averager
	blockProcess             metric.Averager

	executorBuildRecorder  executor.Metrics
	executorVerifyRecorder executor.Metrics
}
