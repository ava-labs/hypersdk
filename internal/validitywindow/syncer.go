// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/hypersdk/internal/emap"
)

// Syncer marks sequential blocks as accepted until it has observed a full validity window
// and signals to the caller that it can begin processing blocks from that block forward.
type Syncer[T emap.Item] struct {
	chainIndex         ChainIndex[T]
	timeValidityWindow *TimeValidityWindow[T]
	getValidityWindow  GetTimeValidityWindowFunc
	initialBlock       ExecutionBlock[T]
}

func NewSyncer[T emap.Item](chainIndex ChainIndex[T], timeValidityWindow *TimeValidityWindow[T], getValidityWindow GetTimeValidityWindowFunc) *Syncer[T] {
	return &Syncer[T]{
		chainIndex:         chainIndex,
		timeValidityWindow: timeValidityWindow,
		getValidityWindow:  getValidityWindow,
	}
}

func (s *Syncer[T]) start(ctx context.Context, lastAcceptedBlock ExecutionBlock[T]) (bool, error) {
	s.initialBlock = lastAcceptedBlock
	// Attempt to backfill the validity window
	var (
		parent             = s.initialBlock
		parents            = []ExecutionBlock[T]{parent}
		seenValidityWindow = false
		validityWindow     = s.getValidityWindow(lastAcceptedBlock.GetTimestamp())
		err                error
	)
	for {
		parent, err = s.chainIndex.GetExecutionBlock(ctx, parent.GetParent())
		if err != nil {
			break // If we can't fetch far enough back or we've gone past genesis, execute what we can
		}
		parents = append(parents, parent)
		seenValidityWindow = lastAcceptedBlock.GetTimestamp()-parent.GetTimestamp() > validityWindow
		if seenValidityWindow {
			break
		}
	}

	s.initialBlock = parents[len(parents)-1]
	if s.initialBlock.GetHeight() == 0 {
		seenValidityWindow = true
	}
	for i := len(parents) - 1; i >= 0; i-- {
		blk := parents[i]
		s.timeValidityWindow.Accept(blk)
	}

	return seenValidityWindow, nil
}

func (s *Syncer[T]) Accept(ctx context.Context, blk ExecutionBlock[T]) (bool, error) {
	if s.initialBlock == nil {
		return s.start(ctx, blk)
	}
	seenValidityWindow := blk.GetTimestamp()-s.initialBlock.GetTimestamp() > s.getValidityWindow(blk.GetTimestamp())
	s.timeValidityWindow.Accept(blk)
	return seenValidityWindow, nil
}
