package merkle

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/smap"
)

const (
	stateInitialSize   = 10_000_000
	pendingInitialSize = 1_000_000
)

type Merkle struct {
	state   *smap.SMap[[]byte]
	mdb     merkledb.MerkleDB
	pending map[string]maybe.Maybe[[]byte]
}

func New(db database.Database, vm VM, cfg Config) (*Merkle, error) {
	mdb, err := merkledb.New(context.TODO(), db, merkledb.Config{
		BranchFactor: cfg.GetStateBranchFactor(),
		// RootGenConcurrency limits the number of goroutines
		// that will be used across all concurrent root generations.
		RootGenConcurrency:          cfg.GetRootGenerationCores(),
		HistoryLength:               cfg.GetStateHistoryLength(),
		ValueNodeCacheSize:          cfg.GetValueNodeCacheSize(),
		IntermediateNodeCacheSize:   cfg.GetIntermediateNodeCacheSize(),
		IntermediateWriteBufferSize: cfg.GetStateIntermediateWriteBufferSize(),
		IntermediateWriteBatchSize:  cfg.GetStateIntermediateWriteBatchSize(),
		Reg:                         vm.GetMerkleRegistry(),
		TraceLevel:                  merkledb.InfoTrace,
		Tracer:                      vm.GetTracer(),
	})
	if err != nil {
		return nil, err
	}
	return &Merkle{
		state:   smap.New[[]byte](stateInitialSize),
		mdb:     mdb,
		pending: make(map[string]maybe.Maybe[[]byte], pendingInitialSize),
	}, nil
}

// TODO: ensure we make view async

func (m *Merkle) Close() error {
	return m.mdb.Close()
}
