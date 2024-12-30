// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

// BlockChainIndex defines the generic on-disk index for the Input block type required
// by the VM.
// BlockChainIndex must serve the last accepted block, it is up to the implementation
// how large of a window of accepted blocks to maintain in its index.
// The VM provides a caching layer on top of BlockChainIndex, so the implementation
// does not need to provide its own caching layer.
type BlockChainIndex[T Block] interface {
	UpdateLastAccepted(ctx context.Context, blk T) error
	GetLastAcceptedHeight(ctx context.Context) (uint64, error)
	GetBlock(ctx context.Context, blkID ids.ID) (T, error)
	GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error)
	GetBlockIDHeight(_ context.Context, blkID ids.ID) (uint64, error)
	GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error)
}

func (v *VM[I, O, A]) MakeChainIndex(
	ctx context.Context,
	chainIndex BlockChainIndex[I],
	outputBlock O,
	acceptedBlock A,
	stateReady bool,
) (*ChainIndex[I, O, A], error) {
	v.inputChainIndex = chainIndex
	lastAcceptedHeight, err := v.inputChainIndex.GetLastAcceptedHeight(ctx)
	if err != nil {
		return nil, err
	}
	inputBlock, err := v.inputChainIndex.GetBlockByHeight(ctx, lastAcceptedHeight)
	if err != nil {
		return nil, err
	}

	var lastAcceptedBlock *StatefulBlock[I, O, A]
	if stateReady {
		v.MarkReady(true)
		lastAcceptedBlock, err = v.reprocessToLastAccepted(ctx, inputBlock, outputBlock, acceptedBlock)
		if err != nil {
			return nil, err
		}
	} else {
		v.MarkReady(false)
		lastAcceptedBlock = NewInputBlock(v.covariantVM, inputBlock)
	}
	v.setLastAccepted(lastAcceptedBlock)
	v.chainIndex = &ChainIndex[I, O, A]{
		covariantVM: v.covariantVM,
	}

	return v.chainIndex, nil
}

func (v *VM[I, O, A]) GetChainIndex() *ChainIndex[I, O, A] {
	return v.chainIndex
}

func (v *VM[I, O, A]) reprocessToLastAccepted(ctx context.Context, inputBlock I, outputBlock O, acceptedBlock A) (*StatefulBlock[I, O, A], error) {
	if inputBlock.Height() < outputBlock.Height() || outputBlock.ID() != acceptedBlock.ID() {
		return nil, fmt.Errorf("invalid initial accepted state (Input = %s, Output = %s, Accepted = %s)", inputBlock, outputBlock, acceptedBlock)
	}

	// Re-process from the last output block, to the last accepted input block
	for inputBlock.Height() > outputBlock.Height() {
		reprocessInputBlock, err := v.inputChainIndex.GetBlockByHeight(ctx, outputBlock.Height()+1)
		if err != nil {
			return nil, err
		}

		outputBlock, err = v.chain.Execute(ctx, outputBlock, reprocessInputBlock)
		if err != nil {
			return nil, err
		}
		acceptedBlock, err = v.chain.AcceptBlock(ctx, acceptedBlock, outputBlock)
		if err != nil {
			return nil, err
		}
	}

	return NewAcceptedBlock(v.covariantVM, inputBlock, outputBlock, acceptedBlock), nil
}

type ChainIndex[I Block, O Block, A Block] struct {
	covariantVM *CovariantVM[I, O, A]
}

func (c *ChainIndex[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (I, error) {
	blk, err := c.covariantVM.GetBlockByHeight(ctx, height)
	if err != nil {
		var emptyBlk I
		return emptyBlk, err
	}
	return blk.Input, nil
}

func (c *ChainIndex[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := c.covariantVM.GetBlock(ctx, blkID)
	if err != nil {
		var emptyBlk I
		return emptyBlk, err
	}
	return blk.Input, nil
}

func (c *ChainIndex[I, O, A]) GetPreferredBlock(ctx context.Context) (O, error) {
	blk, err := c.covariantVM.GetBlock(ctx, c.covariantVM.preferredBlkID)
	if err != nil {
		var emptyBlk O
		return emptyBlk, err
	}
	return blk.Output, nil
}

func (c *ChainIndex[I, O, A]) GetLastAccepted(context.Context) A {
	return c.covariantVM.lastAcceptedBlock.Accepted
}
