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

// startStateSync marks the VM as "not ready" so that blocks are verified / accepted vaccuously
// in DynamicStateSync mode until FinishStateSync is called.
func (v *VM[I, O, A]) startStateSync(ctx context.Context, block I) error {
	if err := v.inputChainIndex.UpdateLastAccepted(ctx, block); err != nil {
		return err
	}
	v.markReady(false)
	v.setLastAccepted(NewInputBlock(v, block))
	return nil
}

// finishStateSync is responsible for setting the last accepted block of the VM after state sync completes.
// Expects the caller to hold the snow ctx lock
func (v *VM[I, O, A]) finishStateSync(ctx context.Context, input I, output O, accepted A) error {
	v.chainLock.Lock()
	defer v.chainLock.Unlock()

	// Cannot call FinishStateSync if already marked as ready and in normal operation
	if v.isReady() {
		return fmt.Errorf("can't finish dynamic state sync from normal operation: %s", input)
	}

	// If the block is already the last accepted block, update the fields and return
	if input.ID() == v.lastAcceptedBlock.ID() {
		v.lastAcceptedBlock.setAccepted(output, accepted)
		v.log.Info("Finishing state sync with original target", zap.Stringer("lastAcceptedBlock", v.lastAcceptedBlock))
		v.markReady(true)
		return nil
	}

	// Dynamic state sync notifies completion async, so the engine may continue to process/accept new blocks
	// before we grab chainLock.
	// This means we must reprocess blocks from the target state sync finished on to the updated last
	// accepted block.
	blk := NewAcceptedBlock(v, input, output, accepted)
	v.log.Info("Finishing state sync with target behind last accepted tip",
		zap.Stringer("target", blk),
		zap.Stringer("lastAcceptedBlock", v.lastAcceptedBlock),
	)
	reprocessBlks, err := v.getExclusiveBlockRange(ctx, blk, v.lastAcceptedBlock)
	if err != nil {
		return fmt.Errorf("failed to get block range while completing state sync: %w", err)
	}
	// include lastAcceptedBlock as the last block to re-process
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
	v.markReady(true)
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
