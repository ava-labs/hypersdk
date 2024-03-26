package merkle

import (
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
)

type VM interface {
	GetMerkleRegistry() *prometheus.Registry
	GetTracer() trace.Tracer
}

type Config interface {
	GetStateBranchFactor() merkledb.BranchFactor
	GetRootGenerationCores() uint
	GetStateHistoryLength() uint
	GetValueNodeCacheSize() uint
	GetIntermediateNodeCacheSize() uint
	GetStateIntermediateWriteBufferSize() uint
	GetStateIntermediateWriteBatchSize() uint
}
