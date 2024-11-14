// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"

	"github.com/ava-labs/hypersdk/internal/validity_window"
)

var ErrInvalidExecutionBlockType = errors.New("invalid execution block type")

// Syncer marks sequential blocks as accepted until it has observed a full validity window
// and signals to the caller that it can begin processing blocks from that block forward.
type Syncer struct {
	chainIndex         validity_window.ChainIndex[*Transaction]
	timeValidityWindow validity_window.TimeValidityWindow[*Transaction]
	ruleFactory        RuleFactory
	initialBlock       *ExecutionBlock
}

func NewSyncer(chainIndex validity_window.ChainIndex[*Transaction], timeValidityWindow validity_window.TimeValidityWindow[*Transaction], ruleFactory RuleFactory) *Syncer {
	return &Syncer{
		chainIndex:         chainIndex,
		timeValidityWindow: timeValidityWindow,
		ruleFactory:        ruleFactory,
	}
}

func (s *Syncer) start(ctx context.Context, lastAcceptedBlock *ExecutionBlock) (bool, error) {
	s.initialBlock = lastAcceptedBlock

	// Attempt to backfill the validity window
	var (
		parent             validity_window.ExecutionBlock[*Transaction] = s.initialBlock
		parents                                                         = []validity_window.ExecutionBlock[*Transaction]{parent}
		seenValidityWindow                                              = false
		err                error
	)
	for {
		parent, err = s.chainIndex.GetExecutionBlock(ctx, parent.Parent())
		if err != nil {
			break // If we can't fetch far enough back or we've gone past genesis, execute what we can
		}
		parents = append(parents, parent)
		seenValidityWindow = lastAcceptedBlock.Tmstmp-parent.Timestamp() > s.ruleFactory.GetRules(lastAcceptedBlock.Tmstmp).GetValidityWindow()
		if seenValidityWindow {
			break
		}
	}

	var ok bool
	s.initialBlock, ok = parents[len(parents)-1].(*ExecutionBlock)
	if !ok {
		return false, ErrInvalidExecutionBlockType
	}
	if s.initialBlock.Hght == 0 {
		seenValidityWindow = true
	}
	for i := len(parents) - 1; i >= 0; i-- {
		blk := parents[i]
		s.timeValidityWindow.Accept(blk)
	}

	return seenValidityWindow, nil
}

func (s *Syncer) Accept(ctx context.Context, blk *ExecutionBlock) (bool, error) {
	if s.initialBlock == nil {
		return s.start(ctx, blk)
	}
	seenValidityWindow := blk.Tmstmp-s.initialBlock.Tmstmp > s.ruleFactory.GetRules(blk.Tmstmp).GetValidityWindow()
	s.timeValidityWindow.Accept(blk)
	return seenValidityWindow, nil
}
