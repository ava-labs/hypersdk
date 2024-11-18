// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/hypersdk/internal/emap"
)

// GetValidityWindowFunc is a callback function provided by the NewSyncer caller, returning the
// validity window duration for the given timestamp.
type GetValidityWindowFunc func(int64) int64

// Syncer marks sequential blocks as accepted until it has observed a full validity window
// and signals to the caller that it can begin processing blocks from that block forward.
type syncer[Container emap.Item] struct {
	chainIndex         ChainIndex[Container]
	timeValidityWindow TimeValidityWindow[Container]
	getValidityWindow  GetValidityWindowFunc
	initialBlock       ExecutionBlock[Container]
}

func NewSyncer[Container emap.Item](chainIndex ChainIndex[Container], timeValidityWindow TimeValidityWindow[Container], getValidityWindow GetValidityWindowFunc) Syncer[Container] {
	return &syncer[Container]{
		chainIndex:         chainIndex,
		timeValidityWindow: timeValidityWindow,
		getValidityWindow:  getValidityWindow,
	}
}

func (s *syncer[Container]) start(ctx context.Context, lastAcceptedBlock ExecutionBlock[Container]) (bool, error) {
	s.initialBlock = lastAcceptedBlock

	// Attempt to backfill the validity window
	var (
		parent             = s.initialBlock
		parents            = []ExecutionBlock[Container]{parent}
		seenValidityWindow = false
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

func (s *syncer[Container]) Accept(ctx context.Context, blk ExecutionBlock[Container]) (bool, error) {
	if s.initialBlock == nil {
		return s.start(ctx, blk)
	}
	seenValidityWindow := blk.Timestamp()-s.initialBlock.Timestamp() > s.getValidityWindow(blk.Timestamp())
	s.timeValidityWindow.Accept(blk)
	return seenValidityWindow, nil
}
