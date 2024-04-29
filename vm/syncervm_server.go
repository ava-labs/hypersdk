// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
)

// GetLastStateSummary returns the latest state summary.
// If no summary is available, [database.ErrNotFound] must be returned.
func (vm *VM) GetLastStateSummary(context.Context) (block.StateSummary, error) {
	summary := chain.NewSyncableBlock(vm.LastAcceptedBlock())
	vm.Logger().Info("Serving syncable block at latest height", zap.Stringer("summary", summary))
	return summary, nil
}

// GetStateSummary implements StateSyncableVM and returns a summary corresponding
// to the provided [height] if the node can serve state sync data for that key.
// If not, [database.ErrNotFound] must be returned.
func (vm *VM) GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error) {
	id, err := vm.GetBlockIDAtHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	block, err := vm.GetStatelessBlock(ctx, id)
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

func (vm *VM) ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error) {
	sb, err := chain.ParseBlock(ctx, bytes, choices.Processing, vm)
	if err != nil {
		return nil, err
	}
	summary := chain.NewSyncableBlock(sb)
	vm.Logger().Info("parsed state summary", zap.Stringer("summary", summary))
	return summary, nil
}
