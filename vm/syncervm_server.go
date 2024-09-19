// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"

	"github.com/ava-labs/hypersdk/chain"
)

// GetLastStateSummary returns the latest state summary.
// If no summary is available, [database.ErrNotFound] must be returned.
func (vm *VM[_]) GetLastStateSummary(context.Context) (block.StateSummary, error) {
	summary := chain.NewSyncableBlock(vm.LastAcceptedBlock())
	vm.Logger().Info("Serving syncable block at latest height", zap.Stringer("summary", summary))
	return summary, nil
}

// GetStateSummary implements StateSyncableVM and returns a summary corresponding
// to the provided [height] if the node can serve state sync data for that key.
// If not, [database.ErrNotFound] must be returned.
func (vm *VM[_]) GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error) {
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

func (vm *VM[T]) ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error) {
	sb, err := chain.ParseBlock[T](ctx, bytes, false, vm)
	if err != nil {
		return nil, err
	}
	summary := chain.NewSyncableBlock(sb)
	vm.Logger().Info("parsed state summary", zap.Stringer("summary", summary))
	return summary, nil
}
