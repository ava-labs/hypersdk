package merkle

import (
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/smap"
)

type Merkle struct {
	state *smap.SMap[[]byte]

	mdb     merkledb.MerkleDB
	pending map[string]maybe.Maybe[[]byte]
}

func NewMerkle() (*Merkle, error) {
	vm.stateDB, err = merkledb.New(ctx, vm.rawStateDB, merkledb.Config{
		BranchFactor: vm.genesis.GetStateBranchFactor(),
		// RootGenConcurrency limits the number of goroutines
		// that will be used across all concurrent root generations.
		RootGenConcurrency:          uint(vm.config.GetRootGenerationCores()),
		HistoryLength:               uint(vm.config.GetStateHistoryLength()),
		ValueNodeCacheSize:          uint(vm.config.GetValueNodeCacheSize()),
		IntermediateNodeCacheSize:   uint(vm.config.GetIntermediateNodeCacheSize()),
		IntermediateWriteBufferSize: uint(vm.config.GetStateIntermediateWriteBufferSize()),
		IntermediateWriteBatchSize:  uint(vm.config.GetStateIntermediateWriteBatchSize()),
		Reg:                         merkleRegistry,
		TraceLevel:                  merkledb.InfoTrace,
		Tracer:                      vm.tracer,
	})
}

func (m *Merkle) Close() error {
}
