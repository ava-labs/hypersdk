// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "context"

// Syncer marks sequential blocks as accepted until it has observed a full validity window
// and signals to the caller that it can begin processing blocks from that block forward.
type Syncer struct {
	chain        *Chain
	initialBlock *ExecutionBlock
}

func NewSyncer(chain *Chain) *Syncer {
	return &Syncer{
		chain: chain,
	}
}

func (s *Syncer) Start(ctx context.Context, lastAcceptedBlock *ExecutionBlock) (bool, error) {
	s.initialBlock = lastAcceptedBlock

	// Attempt to backfill the validity window
	parent := s.initialBlock
	parents := []*ExecutionBlock{parent}
	seenValidityWindow := false
	for {
		parent, err := s.chain.chainIndex.GetExecutionBlock(ctx, parent.Prnt)
		if err != nil {
			break // If we can't fetch far enough back, execute what we can
		}
		parents = append(parents, parent)
		seenValidityWindow = parent.Tmstmp-s.initialBlock.Tmstmp > s.chain.ruleFactory.GetRules(s.initialBlock.Tmstmp).GetValidityWindow()
		if seenValidityWindow {
			break
		}
	}

	s.initialBlock = parents[len(parents)-1]
	for i := len(parents) - 1; i >= 0; i-- {
		blk := parents[i]
		_, err := s.AcceptBlock(ctx, blk)
		if err != nil {
			return false, err
		}
	}

	return seenValidityWindow, nil
}

func (s *Syncer) AcceptBlock(ctx context.Context, blk *ExecutionBlock) (bool, error) {
	seenValidityWindow := blk.Tmstmp-s.initialBlock.Tmstmp > s.chain.ruleFactory.GetRules(blk.Tmstmp).GetValidityWindow()
	return seenValidityWindow, s.chain.AcceptBlock(ctx, blk)
}
