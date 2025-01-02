// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"go.uber.org/zap"
)

var _ block.StateSyncableVM = (*VM[Block, Block, Block])(nil)

func (a *Application[I, O, A]) WithStateSyncableVM(stateSyncableVM block.StateSyncableVM) {
	a.StateSyncableVM = stateSyncableVM
}

// StartStateSync marks the VM as "not ready" so that blocks are verified / accepted vaccuously
// in DynamicStateSync mode until FinishStateSync is called.
func (v *VM[I, O, A]) StartStateSync(ctx context.Context, block I) error {
	if err := v.inputChainIndex.UpdateLastAccepted(ctx, block); err != nil {
		return err
	}
	v.MarkReady(false)
	v.setLastAccepted(NewInputBlock(v.covariantVM, block))
	return nil
}

// FinishStateSync is responsible for setting the last accepted block of the VM after state sync completes.
// Expects the caller to hold the snow ctx lock
func (v *VM[I, O, A]) FinishStateSync(ctx context.Context, input I, output O, accepted A) error {
	v.chainLock.Lock()
	defer v.chainLock.Unlock()

	// Cannot call FinishStateSync if already marked as ready and in normal operation
	if v.Ready() {
		return fmt.Errorf("can't finish dynamic state sync from normal operation: %s", input)
	}

	// If the block is already the last accepted block, update the fields and return
	if input.ID() == v.lastAcceptedBlock.ID() {
		v.lastAcceptedBlock.setAccepted(output, accepted)
		v.log.Info("Finishing state sync with original target", zap.Stringer("lastAcceptedBlock", v.lastAcceptedBlock))
		v.MarkReady(true)
		return nil
	}

	// Dynamic state sync notifies completion async, so we may have verified/accepted new blocks in
	// the interim (before successfully grabbing the lock).
	// Create the block and reprocess all blocks in the range (blk, lastAcceptedBlock]
	blk := NewAcceptedBlock(v.covariantVM, input, output, accepted)
	v.log.Info("Finishing state sync with target behind last accepted tip",
		zap.Stringer("target", blk),
		zap.Stringer("lastAcceptedBlock", v.lastAcceptedBlock),
	)
	reprocessBlks, err := v.covariantVM.getExclusiveBlockRange(ctx, blk, v.lastAcceptedBlock)
	if err != nil {
		return fmt.Errorf("failed to get block range while completing state sync: %w", err)
	}
	// Guarantee that parent is fully populated, so we can correctly verify/accept each
	// block up to and including the last accepted block.
	reprocessBlks = append(reprocessBlks, v.lastAcceptedBlock)
	parent := blk
	v.log.Info("Reprocessing blocks from target to last accepted tip",
		zap.Stringer("target", blk),
		zap.Stringer("lastAcceptedBlock", v.lastAcceptedBlock),
		zap.Int("numBlocks", len(reprocessBlks)),
	)
	start := time.Now()
	for _, reprocessBlk := range reprocessBlks {
		if err := reprocessBlk.verify(ctx, parent.Output); err != nil {
			return fmt.Errorf("failed to finish state sync while verifying block %s in range (%s, %s): %w", reprocessBlk, blk, v.lastAcceptedBlock, err)
		}
		if err := reprocessBlk.accept(ctx, parent.Accepted); err != nil {
			return fmt.Errorf("failed to finish state sync while accepting block %s in range (%s, %s): %w", reprocessBlk, blk, v.lastAcceptedBlock, err)
		}
		parent = reprocessBlk
	}

	v.log.Info("Finished reprocessing blocks", zap.Duration("duration", time.Since(start)))
	v.setLastAccepted(v.lastAcceptedBlock)
	v.MarkReady(true)
	return nil
}

func (v *VM[I, O, A]) StateSyncEnabled(ctx context.Context) (bool, error) {
	return v.app.StateSyncableVM.StateSyncEnabled(ctx)
}

func (v *VM[I, O, A]) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.app.StateSyncableVM.GetOngoingSyncStateSummary(ctx)
}

func (v *VM[I, O, A]) GetLastStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.app.StateSyncableVM.GetLastStateSummary(ctx)
}

func (v *VM[I, O, A]) ParseStateSummary(ctx context.Context, summaryBytes []byte) (block.StateSummary, error) {
	return v.app.StateSyncableVM.ParseStateSummary(ctx, summaryBytes)
}

func (v *VM[I, O, A]) GetStateSummary(ctx context.Context, summaryHeight uint64) (block.StateSummary, error) {
	return v.app.StateSyncableVM.GetStateSummary(ctx, summaryHeight)
}
