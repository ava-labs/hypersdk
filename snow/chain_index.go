// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

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
	v.chainIndex = chainIndex
	lastAcceptedHeight, err := v.chainIndex.GetLastAcceptedHeight(ctx)
	if err != nil {
		return nil, err
	}
	inputBlock, err := v.chainIndex.GetBlockByHeight(ctx, lastAcceptedHeight)
	if err != nil {
		return nil, err
	}

	var lastAcceptedBlock *StatefulBlock[I, O, A]
	if stateReady {
		lastAcceptedBlock, err = v.reprocessToLastAccepted(ctx, inputBlock, outputBlock, acceptedBlock)
		if err != nil {
			return nil, err
		}
	} else {
		lastAcceptedBlock = NewInputBlock(v.covariantVM, inputBlock)
		v.app.Ready.MarkNotReady()
	}
	v.setLastAccepted(lastAcceptedBlock)

	return &ChainIndex[I, O, A]{
		covariantVM: v.covariantVM,
	}, nil
}

func (v *VM[I, O, A]) reprocessToLastAccepted(ctx context.Context, inputBlock I, outputBlock O, acceptedBlock A) (*StatefulBlock[I, O, A], error) {
	if inputBlock.Height() < outputBlock.Height() || outputBlock.ID() != acceptedBlock.ID() {
		return nil, fmt.Errorf("invalid initial accepted state (Input = %s, Output = %s, Accepted = %s)", inputBlock, outputBlock, acceptedBlock)
	}

	// Re-process from the last output block, to the last accepted input block
	for inputBlock.Height() > outputBlock.Height() {
		reprocessInputBlock, err := v.chainIndex.GetBlockByHeight(ctx, outputBlock.Height()+1)
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

func (c *ChainIndex[I, O, A]) GetLastAccepted(ctx context.Context) A {
	return c.covariantVM.lastAcceptedBlock.Accepted
}
