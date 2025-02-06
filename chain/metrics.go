// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/internal/executor"
)

const namespace = "chain"

type ChainMetrics struct {
	txsBuilt    prometheus.Counter
	txsVerified prometheus.Counter
	txsAccepted prometheus.Counter

	rootCalculatedCount prometheus.Counter
	rootCalculatedSum   prometheus.Gauge
	waitRootCount       prometheus.Counter
	waitRootSum         prometheus.Gauge
	waitSignaturesCount prometheus.Counter
	waitSignaturesSum   prometheus.Gauge

	buildCapped     prometheus.Counter
	emptyBlockBuilt prometheus.Counter
	clearedMempool  prometheus.Counter

	stateChanges    prometheus.Counter
	stateOperations prometheus.Counter

	executorBuildBlocked     prometheus.Counter
	executorBuildExecutable  prometheus.Counter
	executorVerifyBlocked    prometheus.Counter
	executorVerifyExecutable prometheus.Counter

	executorBuildRecorder  executor.Metrics
	executorVerifyRecorder executor.Metrics
}

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

func NewMetrics(reg *prometheus.Registry) (*ChainMetrics, error) {
	m := &ChainMetrics{
		txsBuilt: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_built",
			Help:      "Total # of txs included within a locally built block",
		}),
		txsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_verified",
			Help:      "Total # of txs verified within block execution",
		}),
		txsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "txs_accepted",
			Help:      "Total # of txs accepted within a block",
		}),
		rootCalculatedCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "root_calculated_count",
			Help:      "Total # of observations of time spent calculating the state root in verify",
		}),
		rootCalculatedSum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "root_calculated_sum",
			Help:      "Sum of time spent calculating the state root in verify",
		}),
		waitRootCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "wait_root_count",
			Help:      "Total # of observations of time spent waiting for the state root in verify",
		}),
		waitRootSum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "wait_root_sum",
			Help:      "Sum of time spent waiting for the state root in verify",
		}),
		waitSignaturesCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "wait_signatures_count",
			Help:      "Total # of observations of time spent waiting for signature verification",
		}),
		waitSignaturesSum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "wait_signatures_sum",
			Help:      "Sum of time spent waiting for signature verification",
		}),
		buildCapped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "build_capped",
			Help:      "number of times build capped by target duration",
		}),
		emptyBlockBuilt: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "empty_block_built",
			Help:      "number of times empty block built",
		}),
		executorBuildBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "executor_build_blocked",
			Help:      "executor tasks blocked during build",
		}),
		executorBuildExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "executor_build_executable",
			Help:      "executor tasks executable during build",
		}),
		executorVerifyBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "executor_verify_blocked",
			Help:      "executor tasks blocked during verify",
		}),
		executorVerifyExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "executor_verify_executable",
			Help:      "executor tasks executable during verify",
		}),
		stateChanges: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "state_changes",
			Help:      "number of state changes",
		}),
		stateOperations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "state_operations",
			Help:      "number of state operations",
		}),
		clearedMempool: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cleared_mempool",
			Help:      "number of times cleared mempool while building",
		}),
	}

	m.executorBuildRecorder = &executorMetrics{blocked: m.executorBuildBlocked, executable: m.executorBuildExecutable}
	m.executorVerifyRecorder = &executorMetrics{blocked: m.executorVerifyBlocked, executable: m.executorVerifyExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		reg.Register(m.txsBuilt),
		reg.Register(m.txsVerified),
		reg.Register(m.txsAccepted),
		reg.Register(m.rootCalculatedCount),
		reg.Register(m.rootCalculatedSum),
		reg.Register(m.waitRootCount),
		reg.Register(m.waitRootSum),
		reg.Register(m.waitSignaturesCount),
		reg.Register(m.waitSignaturesSum),
		reg.Register(m.buildCapped),
		reg.Register(m.emptyBlockBuilt),
		reg.Register(m.executorBuildBlocked),
		reg.Register(m.executorBuildExecutable),
		reg.Register(m.executorVerifyBlocked),
		reg.Register(m.executorVerifyExecutable),
		reg.Register(m.stateChanges),
		reg.Register(m.stateOperations),
		reg.Register(m.clearedMempool),
	)
	return m, errs.Err
}
