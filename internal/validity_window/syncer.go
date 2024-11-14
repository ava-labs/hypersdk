// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validity_window

import (
	"context"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type GetValidityWindowFunc func(int64) int64

// Syncer marks sequential blocks as accepted until it has observed a full validity window
// and signals to the caller that it can begin processing blocks from that block forward.
type syncer[TxnTypePtr emap.Item] struct {
	chainIndex         ChainIndex[TxnTypePtr]
	timeValidityWindow TimeValidityWindow[TxnTypePtr]
	getValidityWindow  GetValidityWindowFunc
	initialBlock       ExecutionBlock[TxnTypePtr]
}

func NewSyncer[TxnTypePtr emap.Item](chainIndex ChainIndex[TxnTypePtr], timeValidityWindow TimeValidityWindow[TxnTypePtr], getValidityWindow GetValidityWindowFunc) Syncer[TxnTypePtr] {
	return &syncer[TxnTypePtr]{
		chainIndex:         chainIndex,
		timeValidityWindow: timeValidityWindow,
		getValidityWindow:  getValidityWindow,
	}
}

func (s *syncer[TxnTypePtr]) start(ctx context.Context, lastAcceptedBlock ExecutionBlock[TxnTypePtr]) (bool, error) {
	s.initialBlock = lastAcceptedBlock

	// Attempt to backfill the validity window
	var (
		parent             ExecutionBlock[TxnTypePtr] = s.initialBlock
		parents                                       = []ExecutionBlock[TxnTypePtr]{parent}
		seenValidityWindow                            = false
		err                error
	)
	for {
		parent, err = s.chainIndex.GetExecutionBlock(ctx, parent.Parent())
		if err != nil {
			break // If we can't fetch far enough back or we've gone past genesis, execute what we can
		}
		parents = append(parents, parent)
		seenValidityWindow = lastAcceptedBlock.Timestamp()-parent.Timestamp() > s.getValidityWindow(lastAcceptedBlock.Timestamp())
		if seenValidityWindow {
			break
		}
	}

	s.initialBlock = parents[len(parents)-1]
	if s.initialBlock.Height() == 0 {
		seenValidityWindow = true
	}
	for i := len(parents) - 1; i >= 0; i-- {
		blk := parents[i]
		s.timeValidityWindow.Accept(blk)
	}

	return seenValidityWindow, nil
}

func (s *syncer[TxnTypePtr]) Accept(ctx context.Context, blk ExecutionBlock[TxnTypePtr]) (bool, error) {
	if s.initialBlock == nil {
		return s.start(ctx, blk)
	}
	seenValidityWindow := blk.Timestamp()-s.initialBlock.Timestamp() > s.getValidityWindow(blk.Timestamp())
	s.timeValidityWindow.Accept(blk)
	return seenValidityWindow, nil
}
