// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/hypersdk/utils"
)

type Assembler struct {
	vm VM
}

func (a *Assembler) AssembleBlock(
	ctx context.Context,
	parent *StatefulBlock,
	timestamp int64,
	blockHeight uint64,
	txs []*Transaction,
) (*StatefulBlock, error) {
	ctx, span := a.vm.Tracer().Start(ctx, "chain.AssembleBlock")
	defer span.End()

	parentView, err := parent.View(ctx, true)
	if err != nil {
		return nil, err
	}
	parentStateRoot, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}

	blk := &StatelessBlock{
		Prnt:      parent.ID(),
		Tmstmp:    timestamp,
		Hght:      blockHeight,
		Txs:       txs,
		StateRoot: parentStateRoot,
	}
	for _, tx := range txs {
		blk.authCounts[tx.Auth.GetTypeID()]++
	}

	blkBytes, err := blk.Marshal()
	if err != nil {
		return nil, err
	}
	b := &StatefulBlock{
		StatelessBlock: blk,
		t:              time.UnixMilli(blk.Tmstmp),
		bytes:          blkBytes,
		accepted:       false,
		vm:             a.vm,
		id:             utils.ToID(blkBytes),
	}
	return b, b.populateTxs(ctx) // TODO: simplify since txs are guaranteed to already be de-duplicated here
}

func (a *Assembler) ExecuteBlock(
	ctx context.Context,
	b *StatefulBlock,
) (*ExecutedBlock, error) {
	ctx, span := a.vm.Tracer().Start(ctx, "chain.ExecuteBlock")
	defer span.End()

	if err := b.Verify(ctx); err != nil {
		return nil, err
	}

	return NewExecutedBlockFromStateful(b), nil
}
