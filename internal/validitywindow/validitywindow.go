// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/emap"
)

var (
	ErrDuplicateContainer            = errors.New("duplicate container")
	ErrExecutionBlockRetrievalFailed = errors.New("unable to retrieve block")
)

type TimeValidityWindow[Container emap.Item] struct {
	log    logging.Logger
	tracer trace.Tracer

	lock                    sync.Mutex
	chainIndex              ChainIndex[Container]
	seen                    *emap.EMap[Container]
	lastAcceptedBlockHeight uint64
}

func NewTimeValidityWindow[Container emap.Item](log logging.Logger, tracer trace.Tracer, chainIndex ChainIndex[Container]) *TimeValidityWindow[Container] {
	return &TimeValidityWindow[Container]{
		log:        log,
		tracer:     tracer,
		chainIndex: chainIndex,
		seen:       emap.NewEMap[Container](),
	}
}

func (v *TimeValidityWindow[Container]) Accept(blk ExecutionBlock[Container]) {
	// Grab the lock before modifiying seen
	v.lock.Lock()
	defer v.lock.Unlock()

	evicted := v.seen.SetMin(blk.Timestamp())
	v.log.Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	v.seen.Add(blk.Txs())
	v.lastAcceptedBlockHeight = blk.Height()
}

func (v *TimeValidityWindow[Container]) VerifyExpiryReplayProtection(
	ctx context.Context,
	blk ExecutionBlock[Container],
	oldestAllowed int64,
) error {
	if blk.Height() <= v.lastAcceptedBlockHeight {
		return nil
	}
	parent, hasBlock, err := v.chainIndex.GetExecutionBlock(ctx, blk.Parent())
	if err != nil {
		return err
	}
	if !hasBlock {
		// if we can't get the block, we won't be able to determine if any of the transaction are duplicate.
		// this is not an expected result, and is equivilient to the case where we cannot perform the validation
		// due to disk issue.
		return ErrExecutionBlockRetrievalFailed
	}

	dup, err := v.isRepeat(ctx, parent, oldestAllowed, blk.Txs(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateContainer)
	}
	// make sure we have no repeats within the block itself.
	// set.Set
	blkTxsIDs := set.NewSet[ids.ID](len(blk.Txs()))
	for _, tx := range blk.Txs() {
		id := tx.GetID()
		if blkTxsIDs.Contains(id) {
			return fmt.Errorf("%w: duplicate in block", ErrDuplicateContainer)
		}
		blkTxsIDs.Add(id)
	}
	return nil
}

func (v *TimeValidityWindow[Container]) IsRepeat(
	ctx context.Context,
	parentBlk ExecutionBlock[Container],
	txs []Container,
	oldestAllowed int64,
) (set.Bits, error) {
	return v.isRepeat(ctx, parentBlk, oldestAllowed, txs, false)
}

func (v *TimeValidityWindow[Container]) isRepeat(
	ctx context.Context,
	ancestorBlk ExecutionBlock[Container],
	oldestAllowed int64,
	txs []Container,
	stop bool,
) (set.Bits, error) {
	marker := set.NewBits()

	_, span := v.tracer.Start(ctx, "Chain.isRepeat")
	defer span.End()

	v.lock.Lock()
	defer v.lock.Unlock()

	var err error
	var hasBlock bool
	for {
		if ancestorBlk.Timestamp() < oldestAllowed {
			return marker, nil
		}

		if ancestorBlk.Height() <= v.lastAcceptedBlockHeight || ancestorBlk.Height() == 0 {
			return v.seen.Contains(txs, marker, stop), nil
		}

		for i, tx := range txs {
			if marker.Contains(i) {
				continue
			}
			if ancestorBlk.Contains(tx.GetID()) {
				marker.Add(i)
				if stop {
					return marker, nil
				}
			}
		}

		ancestorBlk, hasBlock, err = v.chainIndex.GetExecutionBlock(ctx, ancestorBlk.Parent())
		if err != nil || !hasBlock {
			return marker, err
		}
	}
}
