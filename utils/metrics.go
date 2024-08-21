package utils

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	TxsSubmitted             prometheus.Counter // includes gossip
	TxsReceived              prometheus.Counter
	SeenTxsReceived          prometheus.Counter
	TxsGossiped              prometheus.Counter
	TxsVerified              prometheus.Counter
	TxsAccepted              prometheus.Counter
	StateChanges             prometheus.Counter
	StateOperations          prometheus.Counter
	AllocKeyOperations       prometheus.Gauge
	WriteKeyOperations       prometheus.Gauge
	BuildCapped              prometheus.Counter
	EmptyBlockBuilt          prometheus.Counter
	ClearedMempool           prometheus.Counter
	DeletedBlocks            prometheus.Counter
	BlocksFromDisk           prometheus.Counter
	BlocksHeightsFromDisk    prometheus.Counter
	ExecutorBuildBlocked     prometheus.Counter
	ExecutorBuildExecutable  prometheus.Counter
	ExecutorVerifyBlocked    prometheus.Counter
	ExecutorVerifyExecutable prometheus.Counter
	MempoolSize              prometheus.Gauge
	BandwidthPrice           prometheus.Gauge
	ComputePrice             prometheus.Gauge
	StorageReadPrice         prometheus.Gauge
	StorageAllocatePrice     prometheus.Gauge
	StorageWritePrice        prometheus.Gauge
	RootCalculated           metric.Averager
	WaitRoot                 metric.Averager
	WaitSignatures           metric.Averager
	BlockBuild               metric.Averager
	BlockParse               metric.Averager
	BlockVerify              metric.Averager
	BlockAccept              metric.Averager
	BlockProcess             metric.Averager

	ExecutorBuildRecorder  executor.Metrics
	ExecutorVerifyRecorder executor.Metrics
}

func NewMetrics() (*prometheus.Registry, *Metrics, error) {
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
		TxsSubmitted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_submitted",
			Help:      "number of txs submitted to vm",
		}),
		TxsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_received",
			Help:      "number of txs received over gossip",
		}),
		SeenTxsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "seen_txs_received",
			Help:      "number of txs received over gossip that we've already seen",
		}),
		TxsGossiped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_gossiped",
			Help:      "number of txs gossiped by vm",
		}),
		TxsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_verified",
			Help:      "number of txs verified by vm",
		}),
		TxsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_accepted",
			Help:      "number of txs accepted by vm",
		}),
		StateChanges: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "state_changes",
			Help:      "number of state changes",
		}),
		StateOperations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "state_operations",
			Help:      "number of state operations",
		}),
		AllocKeyOperations: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tstate",
			Name:      "alloc_key_operations",
			Help:      "number of allocates operations",
		}),
		WriteKeyOperations: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tstate",
			Name:      "write_key_operations",
			Help:      "number of writes operations",
		}),
		BuildCapped: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "build_capped",
			Help:      "number of times build capped by target duration",
		}),
		EmptyBlockBuilt: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "empty_block_built",
			Help:      "number of times empty block built",
		}),
		ClearedMempool: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "cleared_mempool",
			Help:      "number of times cleared mempool while building",
		}),
		DeletedBlocks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "deleted_blocks",
			Help:      "number of blocks deleted",
		}),
		BlocksFromDisk: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "blocks_from_disk",
			Help:      "number of blocks attempted to load from disk",
		}),
		BlocksHeightsFromDisk: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "block_heights_from_disk",
			Help:      "number of block heights attempted to load from disk",
		}),
		ExecutorBuildBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_build_blocked",
			Help:      "executor tasks blocked during build",
		}),
		ExecutorBuildExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_build_executable",
			Help:      "executor tasks executable during build",
		}),
		ExecutorVerifyBlocked: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_verify_blocked",
			Help:      "executor tasks blocked during verify",
		}),
		ExecutorVerifyExecutable: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "executor_verify_executable",
			Help:      "executor tasks executable during verify",
		}),
		MempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "number of transactions in the mempool",
		}),
		BandwidthPrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "bandwidth_price",
			Help:      "unit price of bandwidth",
		}),
		ComputePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "compute_price",
			Help:      "unit price of compute",
		}),
		StorageReadPrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_read_price",
			Help:      "unit price of storage reads",
		}),
		StorageAllocatePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_create_price",
			Help:      "unit price of storage creates",
		}),
		StorageWritePrice: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "storage_modify_price",
			Help:      "unit price of storage modifications",
		}),
		RootCalculated: rootCalculated,
		WaitRoot:       waitRoot,
		WaitSignatures: waitSignatures,
		BlockBuild:     blockBuild,
		BlockParse:     blockParse,
		BlockVerify:    blockVerify,
		BlockAccept:    blockAccept,
		BlockProcess:   blockProcess,
	}
	// m.ExecutorBuildRecorder = &vm.ExecutorMetrics{blocked: m.executorBuildBlocked, executable: m.executorBuildExecutable}
	// m.ExecutorVerifyRecorder = &vm.ExecutorMetrics{blocked: m.executorVerifyBlocked, executable: m.executorVerifyExecutable}

	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.TxsSubmitted),
		r.Register(m.TxsReceived),
		r.Register(m.SeenTxsReceived),
		r.Register(m.TxsGossiped),
		r.Register(m.TxsVerified),
		r.Register(m.TxsAccepted),
		r.Register(m.StateChanges),
		r.Register(m.StateOperations),
		r.Register(m.AllocKeyOperations),
		r.Register(m.WriteKeyOperations),
		r.Register(m.MempoolSize),
		r.Register(m.BuildCapped),
		r.Register(m.EmptyBlockBuilt),
		r.Register(m.ClearedMempool),
		r.Register(m.DeletedBlocks),
		r.Register(m.BlocksFromDisk),
		r.Register(m.BlocksHeightsFromDisk),
		r.Register(m.ExecutorBuildBlocked),
		r.Register(m.ExecutorBuildExecutable),
		r.Register(m.ExecutorVerifyBlocked),
		r.Register(m.ExecutorVerifyExecutable),
		r.Register(m.BandwidthPrice),
		r.Register(m.ComputePrice),
		r.Register(m.StorageReadPrice),
		r.Register(m.StorageAllocatePrice),
		r.Register(m.StorageWritePrice),
	)
	return r, m, errs.Err
}

func (m *Metrics) UpdateWriteOperations(oldValue uint16, newValue uint16) {
	if newValue > oldValue {
		m.WriteKeyOperations.Add(float64(newValue - oldValue))
	} else if oldValue > newValue {
		m.WriteKeyOperations.Sub(float64(oldValue - newValue))
	}
}

func (m *Metrics) UpdateAllocOperations(oldValue uint16, newValue uint16) {
	if newValue > oldValue {
		m.AllocKeyOperations.Add(float64(newValue - oldValue))
	} else if oldValue > newValue {
		m.AllocKeyOperations.Sub(float64(oldValue - newValue))
	}
}
