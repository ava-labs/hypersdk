// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package netextensions

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/hypersdk/chain"
	"go.uber.org/zap"
)

var _ block.StateSyncableVM = (*StateSyncerServer)(nil)

type BlockIDAtHeightGetter interface {
	GetBlockIDAtHeight(ctx context.Context, height uint64) (ids.ID, error)
}

type VM interface {
	chain.VM
	BlockIDAtHeightGetter
}

type StateSyncerServer struct {
	VM
}

func NewStateSyncerServer(vm VM) *StateSyncerServer {
	return &StateSyncerServer{
		VM: vm,
	}
}

// GetLastStateSummary returns the latest state summary.
// If no summary is available, [database.ErrNotFound] must be returned.
func (vm *StateSyncerServer) GetLastStateSummary(context.Context) (block.StateSummary, error) {
	summary := chain.NewSyncableBlock(vm.LastAcceptedBlock())
	vm.Logger().Info("Serving syncable block at latest height", zap.Stringer("summary", summary))
	return summary, nil
}

// GetStateSummary implements StateSyncableVM and returns a summary corresponding
// to the provided [height] if the node can serve state sync data for that key.
// If not, [database.ErrNotFound] must be returned.
func (vm *StateSyncerServer) GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error) {
	id, err := vm.GetBlockIDAtHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	block, err := vm.GetStatefulBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	summary := chain.NewSyncableBlock(block)
	vm.Logger().Info("Serving syncable block at requested height",
		zap.Uint64("height", height),
		zap.Stringer("summary", summary),
	)
	return summary, nil
}

func (vm *StateSyncerServer) ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error) {
	sb, err := chain.ParseBlock(ctx, bytes, false, vm)
	if err != nil {
		return nil, err
	}
	summary := chain.NewSyncableBlock(sb)
	vm.Logger().Info("parsed state summary", zap.Stringer("summary", summary))
	return summary, nil
}

func (*StateSyncerServer) StateSyncEnabled(context.Context) (bool, error) {
	// We always start the state syncer and may fallback to normal bootstrapping
	// if we are close to tip.
	//
	// There is no way to trigger a full bootstrap from genesis.
	return true, nil
}

func (*StateSyncerServer) GetOngoingSyncStateSummary(
	context.Context,
) (block.StateSummary, error) {
	// Because the history of MerkleDB change proofs tends to be short, we always
	// restart syncing from scratch.
	//
	// This is unlike other DB implementations where roots are persisted
	// indefinitely (and it means we can continue from where we left off).
	return nil, database.ErrNotFound
}
