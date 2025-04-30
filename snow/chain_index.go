// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"

	"github.com/ava-labs/hypersdk/event"
)

// ChainIndex defines the generic on-disk index for the Input block type required
// by the VM.
// ChainIndex must serve the last accepted block, it is up to the implementation
// how large of a window of accepted blocks to maintain in its index.
// The VM provides a caching layer on top of ChainIndex, so the implementation
// does not need to provide its own caching layer.
type ChainIndex[T Block] interface {
	// UpdateLastAccepted updates the chain's last accepted block record and manages block storage.
	// It:
	// 1. Persists the new last accepted block
	// 2. Maintains cross-reference indices between block IDs and heights
	// 3. Prunes old blocks beyond the retention window
	// 4. Periodically triggers database compaction to reclaim space
	// This function is called whenever a block is accepted by consensus
	UpdateLastAccepted(ctx context.Context, blk T) error

	// GetLastAcceptedHeight returns the height of the last accepted block
	GetLastAcceptedHeight(ctx context.Context) (uint64, error)

	// GetBlock returns the block with the given ID
	GetBlock(ctx context.Context, blkID ids.ID) (T, error)

	// GetBlockIDAtHeight returns the ID of the block at the given height
	GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error)

	// GetBlockIDHeight returns the height of the block with the given ID
	GetBlockIDHeight(ctx context.Context, blkID ids.ID) (uint64, error)

	// GetBlockByHeight returns the block at the given height
	GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error)
}

// makeConsensusIndex re-processes blocks from the output/accepted block to the latest possible
// input block that has been added to the chain index if state ready is true.
// Otherwise, initialize an incomplete consensus index that will be initialized when state sync
// finishes.
//
// If the VM provides a last accepted block <= the last processed block (which should always be the case)
// then the VM will re-process blocks from at least as far back as the last processed block and guarantee
// event notifications for all downstream consumers.
func (v *VM[I, O, A]) makeConsensusIndex(
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
		v.ready = true
		lastAcceptedBlock, err = v.reprocessFromOutputToInput(ctx, inputBlock, outputBlock, acceptedBlock)
		if err != nil {
			return err
		}
		v.setLastProcessed(lastAcceptedBlock)
	} else {
		v.ready = false
		lastAcceptedBlock = NewInputBlock(v, inputBlock)
	}
	v.setLastAccepted(lastAcceptedBlock)
	v.preferredBlkID = lastAcceptedBlock.ID()
	v.consensusIndex = &ConsensusIndex[I, O, A]{v}

	return nil
}

// GetConsensusIndex returns the consensus index exposed to the application. The consensus index is created during chain initialization
// and is exposed here for testing.
func (v *VM[I, O, A]) GetConsensusIndex() *ConsensusIndex[I, O, A] {
	return v.consensusIndex
}

// reprocessFromOutputToInput re-processes blocks from output/accepted to align with the supplied input block.
// assumes that outputBlock and acceptedBlock represent the same block and that all blocks in the range
// [output/accepted, input] have been added to the inputChainIndex.
func (v *VM[I, O, A]) reprocessFromOutputToInput(ctx context.Context, targetInputBlock I, outputBlock O, acceptedBlock A) (*StatefulBlock[I, O, A], error) {
	if targetInputBlock.GetHeight() < outputBlock.GetHeight() || outputBlock.GetID() != acceptedBlock.GetID() {
		return nil, fmt.Errorf("invalid initial accepted state (Input = %s, Output = %s, Accepted = %s)", targetInputBlock, outputBlock, acceptedBlock)
	}

	// Re-process from the last output block, to the last accepted input block
	for targetInputBlock.GetHeight() > outputBlock.GetHeight() {
		reprocessInputBlock, err := v.inputChainIndex.GetBlockByHeight(ctx, outputBlock.GetHeight()+1)
		if err != nil {
			return nil, err
		}

		outputBlock, err = v.chain.VerifyBlock(ctx, outputBlock, reprocessInputBlock)
		if err != nil {
			return nil, err
		}
		if err := event.NotifyAll[O](ctx, outputBlock, v.verifiedSubs...); err != nil {
			return nil, fmt.Errorf("failed to notify verified subs during re-processing: %w", err)
		}
		acceptedBlock, err = v.chain.AcceptBlock(ctx, acceptedBlock, outputBlock)
		if err != nil {
			return nil, err
		}
		if err := event.NotifyAll[A](ctx, acceptedBlock, v.acceptedSubs...); err != nil {
			return nil, fmt.Errorf("failed to notify accepted subs during re-processing: %w", err)
		}
	}

	return NewAcceptedBlock(v, targetInputBlock, outputBlock, acceptedBlock), nil
}

// ConsensusIndex provides access to the current consensus state while offering
// type-safety for blocks in different stages of processing.
//
// It serves two main purposes:
//
//  1. Provides developers with access to the caching layer built into the VM,
//     eliminating the need to implement separate caching.
//  2. Offers specialized accessors to blocks at different points in the consensus frontier:
//     - GetLastAccepted() returns the block in its Accepted (A) state, which is guaranteed
//     to be fully committed to the chain.
//     - GetPreferredBlock() returns the block in its Output (O) state, representing
//     the current preference that has been verified but may not yet be accepted.
//
// This type-safe approach ensures developers always have the appropriate block representation
// based on its consensus status. For instance, a preferred block is only guaranteed to have
// reached the Output state, while the last accepted block is guaranteed to have reached
// the Accepted state with all its state transitions finalized.
type ConsensusIndex[I Block, O Block, A Block] struct {
	vm *VM[I, O, A]
}

// GetBlockByHeight retrieves the block at the specified height
func (c *ConsensusIndex[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (I, error) {
	blk, err := c.vm.GetBlockByHeight(ctx, height)
	if err != nil {
		return utils.Zero[I](), err
	}
	return blk.Input, nil
}

// GetBlock fetches the input block of the given block ID
func (c *ConsensusIndex[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := c.vm.GetBlock(ctx, blkID)
	if err != nil {
		return utils.Zero[I](), err
	}
	return blk.Input, nil
}

// GetPreferredBlock returns the output block of the current preference.
//
// Prior to dynamic state sync, GetPreferredBlock will return an error because the preference
// will not have been verified.
// After completing dynamic state sync, all outstanding processing blocks will be verified.
// However, there's an edge case where the node may have vacuously verified an invalid block
// during dynamic state sync, such that the preferred block is invalid and its output is
// empty.
// Consensus should guarantee that we do not accept such a block even if it's preferred as
// long as a majority of validators are correct.
// After outstanding processing blocks have been Accepted/Rejected, the preferred block
// will be verified and the output will be available.
func (c *ConsensusIndex[I, O, A]) GetPreferredBlock(ctx context.Context) (O, error) {
	c.vm.metaLock.Lock()
	preference := c.vm.preferredBlkID
	c.vm.metaLock.Unlock()

	blk, err := c.vm.GetBlock(ctx, preference)
	if err != nil {
		return utils.Zero[O](), err
	}
	if !blk.verified {
		return utils.Zero[O](), fmt.Errorf("preferred block %s has not been verified", blk)
	}
	return blk.Output, nil
}

// GetLastAccepted returns the last accepted block of the chain.
//
// If the chain is mid dynamic state sync, GetLastAccepted will return an error
// because the last accepted block will not be populated.
func (c *ConsensusIndex[I, O, A]) GetLastAccepted(context.Context) (A, error) {
	c.vm.metaLock.Lock()
	defer c.vm.metaLock.Unlock()

	lastAccepted := c.vm.lastProcessedBlock

	if lastAccepted == nil || !lastAccepted.accepted {
		return utils.Zero[A](), fmt.Errorf("last accepted block %s has not been populated", lastAccepted)
	}
	return lastAccepted.Accepted, nil
}
