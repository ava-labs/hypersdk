// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/internal/executor"
)

type chainMetrics struct {
	txsVerified prometheus.Counter
	txsAccepted prometheus.Counter

	rootCalculated metric.Averager
	waitRoot       metric.Averager
	waitSignatures metric.Averager

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

func newMetrics() (*prometheus.Registry, *chainMetrics, error) {
	r := prometheus.NewRegistry()

	rootCalculated, err := metric.NewAverager(
		"chain_root_calculated",
		"time spent calculating the state root in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitRoot, err := metric.NewAverager(
		"chain_wait_root",
		"time spent waiting for root calculation in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitSignatures, err := metric.NewAverager(
		"chain_wait_signatures",
		"time spent waiting for signature verification in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}

	m := &chainMetrics{
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
		txsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "txs_verified",
			Help:      "number of txs verified by chain",
		}),
		txsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "txs_accepted",
			Help:      "number of txs accepted by chain",
		}),
		rootCalculated: rootCalculated,
		waitRoot:       waitRoot,
		waitSignatures: waitSignatures,
	}

	m.executorBuildRecorder = &executorMetrics{blocked: m.executorBuildBlocked, executable: m.executorBuildExecutable}
	m.executorVerifyRecorder = &executorMetrics{blocked: m.executorVerifyBlocked, executable: m.executorVerifyExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.buildCapped),
		r.Register(m.emptyBlockBuilt),
		r.Register(m.executorBuildBlocked),
		r.Register(m.executorBuildExecutable),
		r.Register(m.executorVerifyBlocked),
		r.Register(m.executorVerifyExecutable),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.clearedMempool),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
	)
	return r, m, errs.Err
}
