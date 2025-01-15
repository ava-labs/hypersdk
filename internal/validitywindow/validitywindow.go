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

var ErrDuplicateContainer = errors.New("duplicate container")

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

	evicted := v.seen.SetMin(blk.GetTimestamp())
	v.log.Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	v.seen.Add(blk.GetContainers())
	v.lastAcceptedBlockHeight = blk.GetHeight()
}

func (v *TimeValidityWindow[Container]) VerifyExpiryReplayProtection(
	ctx context.Context,
	blk ExecutionBlock[Container],
	oldestAllowed int64,
) error {
	if blk.GetHeight() <= v.lastAcceptedBlockHeight {
		return nil
	}
	parent, err := v.chainIndex.GetExecutionBlock(ctx, blk.GetParent())
	if err != nil {
		return err
	}

	dup, err := v.isRepeat(ctx, parent, oldestAllowed, blk.GetContainers(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: duplicate bytes %q for %d txs", ErrDuplicateContainer, dup.Bytes(), len(blk.GetContainers()))
	}
	// make sure we have no repeats within the block itself.
	blkContainerIDs := set.NewSet[ids.ID](len(blk.GetContainers()))
	for _, container := range blk.GetContainers() {
		id := container.GetID()
		if blkContainerIDs.Contains(id) {
			return fmt.Errorf("%w: duplicate in block", ErrDuplicateContainer)
		}
		blkContainerIDs.Add(id)
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
	containers []Container,
	stop bool,
) (set.Bits, error) {
	marker := set.NewBits()

	_, span := v.tracer.Start(ctx, "Chain.isRepeat")
	defer span.End()

	v.lock.Lock()
	defer v.lock.Unlock()

	var err error
	for {
		if ancestorBlk.GetTimestamp() < oldestAllowed {
			return marker, nil
		}

		if ancestorBlk.GetHeight() <= v.lastAcceptedBlockHeight || ancestorBlk.GetHeight() == 0 {
			return v.seen.Contains(containers, marker, stop), nil
		}

		for i, container := range containers {
			if marker.Contains(i) {
				continue
			}
			if ancestorBlk.Contains(container.GetID()) {
				marker.Add(i)
				if stop {
					return marker, nil
				}
			}
		}

		ancestorBlk, err = v.chainIndex.GetExecutionBlock(ctx, ancestorBlk.GetParent())
		if err != nil {
			return marker, err
		}
	}
}
