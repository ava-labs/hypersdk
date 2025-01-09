// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"
)

// ChainIndex defines the generic on-disk index for the Input block type required
// by the VM.
// ChainIndex must serve the last accepted block, it is up to the implementation
// how large of a window of accepted blocks to maintain in its index.
// The VM provides a caching layer on top of ChainIndex, so the implementation
// does not need to provide its own caching layer.
type ChainIndex[T Block] interface {
	UpdateLastAccepted(ctx context.Context, blk T) error
	GetLastAcceptedHeight(ctx context.Context) (uint64, error)
	GetBlock(ctx context.Context, blkID ids.ID) (T, error)
	GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error)
	GetBlockIDHeight(_ context.Context, blkID ids.ID) (uint64, error)
	GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error)
}

func (v *vm[I, O, A]) makeConsensusIndex(
	ctx context.Context,
	chainIndex ChainIndex[I],
	outputBlock O,
	acceptedBlock A,
	stateReady bool,
) error {
	v.inputChainIndex = chainIndex
	lastAcceptedHeight, err := v.inputChainIndex.GetLastAcceptedHeight(ctx)
	if err != nil {
		return err
	}
	inputBlock, err := v.inputChainIndex.GetBlockByHeight(ctx, lastAcceptedHeight)
	if err != nil {
		return err
	}

	var lastAcceptedBlock *StatefulBlock[I, O, A]
	if stateReady {
		v.markReady(true)
		lastAcceptedBlock, err = v.reprocessToLastAccepted(ctx, inputBlock, outputBlock, acceptedBlock)
		if err != nil {
			return err
		}
	} else {
		v.markReady(false)
		lastAcceptedBlock = NewInputBlock(v, inputBlock)
	}
	v.setLastAccepted(lastAcceptedBlock)
	v.preferredBlkID = lastAcceptedBlock.ID()
	v.consensusIndex = &ConsensusIndex[I, O, A]{v}

	return nil
}

// GetConsensusIndex returns the consensus index exposed to the application. The consensus index is created during chain initialization
// and is exposed here for testing.
func (v *vm[I, O, A]) GetConsensusIndex() *ConsensusIndex[I, O, A] {
	return v.consensusIndex
}

func (v *vm[I, O, A]) reprocessToLastAccepted(ctx context.Context, inputBlock I, outputBlock O, acceptedBlock A) (*StatefulBlock[I, O, A], error) {
	if inputBlock.Height() < outputBlock.Height() || outputBlock.ID() != acceptedBlock.ID() {
		return nil, fmt.Errorf("invalid initial accepted state (Input = %s, Output = %s, Accepted = %s)", inputBlock, outputBlock, acceptedBlock)
	}

	// Re-process from the last output block, to the last accepted input block
	for inputBlock.Height() > outputBlock.Height() {
		reprocessInputBlock, err := v.inputChainIndex.GetBlockByHeight(ctx, outputBlock.Height()+1)
		if err != nil {
			return nil, err
		}

		outputBlock, err = v.chain.VerifyBlock(ctx, outputBlock, reprocessInputBlock)
		if err != nil {
			return nil, err
		}
		acceptedBlock, err = v.chain.AcceptBlock(ctx, acceptedBlock, outputBlock)
		if err != nil {
			return nil, err
		}
	}

	return NewAcceptedBlock(v, inputBlock, outputBlock, acceptedBlock), nil
}

// ConsensusIndex provides a wrapper around the VM, which enables the chain developer to share the
// caching layer provided by the VM and used in the consensus engine.
// The ConsensusIndex additionally provides access to the accepted/preferred frontier by providing
// accessors to the latest type of the frontier.
// ie. last accepted block is guaranteed to have Accepted type available, whereas the preferred block
// is only guaranteed to have the Output type available.
type ConsensusIndex[I Block, O Block, A Block] struct {
	vm *vm[I, O, A]
}

func (c *ConsensusIndex[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (I, error) {
	blk, err := c.vm.GetBlockByHeight(ctx, height)
	if err != nil {
		var emptyBlk I
		return emptyBlk, err
	}
	return blk.Input, nil
}

func (c *ConsensusIndex[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := c.vm.GetBlock(ctx, blkID)
	if err != nil {
		var emptyBlk I
		return emptyBlk, err
	}
	return blk.Input, nil
}

func (c *ConsensusIndex[I, O, A]) GetPreferredBlock(ctx context.Context) (O, error) {
	c.vm.chainLock.Lock()
	defer c.vm.chainLock.Unlock()

	var emptyOutputBlk O
	blk, err := c.vm.GetBlock(ctx, c.vm.preferredBlkID)
	if err != nil {
		return emptyOutputBlk, err
	}

	if !blk.verified {
		// The block may not be populated if we are transitioning from dynamic state sync.
		// This is jank as hell.
		// To handle this, we re-process from the last verified ancestor to the preferred
		// block.
		// If the preferred block is invalid (which can happen if a malicious peer sends us
		// an invalid block and we hit a poll that causes us to set it to our preference),
		// then we will return an error.
		c.vm.log.Info("Reprocessing to preferred block",
			zap.Stringer("lastAccepted", c.vm.lastAcceptedBlock),
			zap.Stringer("preferred", blk),
		)
		if err := blk.innerVerify(ctx); err != nil {
			return emptyOutputBlk, fmt.Errorf("failed to verify preferred block %s: %w", blk, err)
		}
		return blk.Output, nil
	}
	return blk.Output, nil
}

func (c *ConsensusIndex[I, O, A]) GetLastAccepted(context.Context) A {
	c.vm.metaLock.Lock()
	defer c.vm.metaLock.Unlock()

	return c.vm.lastAcceptedBlock.Accepted
}
