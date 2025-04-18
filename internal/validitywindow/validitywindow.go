// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/emap"
)

var (
	_                     Interface[emap.Item] = (*TimeValidityWindow[emap.Item])(nil)
	ErrDuplicateContainer                      = errors.New("duplicate container")
	ErrMisalignedTime                          = errors.New("misaligned time")
	ErrTimestampExpired                        = errors.New("declared timestamp expired")
	ErrFutureTimestamp                         = errors.New("declared timestamp too far in the future")
)

type GetTimeValidityWindowFunc func(timestamp int64) int64

type Block interface {
	GetID() ids.ID
	GetParent() ids.ID
	GetTimestamp() int64
	GetHeight() uint64
	GetBytes() []byte
}

type ExecutionBlock[T emap.Item] interface {
	Block
	GetContainers() []T
	Contains(ids.ID) bool
}

type ChainIndex[T emap.Item] interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (ExecutionBlock[T], error)
}

type Interface[T emap.Item] interface {
	Accept(blk ExecutionBlock[T])
	VerifyExpiryReplayProtection(ctx context.Context, blk ExecutionBlock[T]) error
	IsRepeat(ctx context.Context, parentBlk ExecutionBlock[T], currentTimestamp int64, containers []T) (set.Bits, error)
}

// TimeValidityWindow is a time-based transaction mechanism that prevents replay attacks.
// It maintains a record of transactions that have been seen within a
// configurable time window and rejects any duplicates.
//
// This component is critical as it:
//  1. Prevents transaction replay attacks
//  2. Enforces double-spend protection
//  3. Provides temporal validation boundaries
//  4. Maintains consensus safety across nodes i.e.;
//     if different nodes had different rules for transaction uniqueness,
//     they would disagree about the state of the blockchain.
//
// TimeValidityWindow builds up on assumption of ChainIndex being up to date
type TimeValidityWindow[T emap.Item] struct {
	log                     logging.Logger
	tracer                  trace.Tracer
	mu                      sync.Mutex
	chainIndex              ChainIndex[T]
	seen                    *emap.EMap[T]
	lastAcceptedBlockHeight uint64
	getTimeValidityWindow   GetTimeValidityWindowFunc
}

// NewTimeValidityWindow constructs TimeValidityWindow and eagerly tries to populate
// a validity window from the tip
func NewTimeValidityWindow[T emap.Item](
	ctx context.Context,
	log logging.Logger,
	tracer trace.Tracer,
	chainIndex ChainIndex[T],
	tip ExecutionBlock[T],
	getTimeValidityWindowF GetTimeValidityWindowFunc,
) *TimeValidityWindow[T] {
	t := &TimeValidityWindow[T]{
		log:                   log,
		tracer:                tracer,
		chainIndex:            chainIndex,
		seen:                  emap.NewEMap[T](),
		getTimeValidityWindow: getTimeValidityWindowF,
	}
	if tip != nil {
		t.populate(ctx, tip)
	}
	return t
}

// Complete will attempt to complete a validity window.
// It returns a boolean that signals if it's ready to reliably prevent replay attacks
func (v *TimeValidityWindow[T]) Complete(ctx context.Context, block ExecutionBlock[T]) bool {
	_, isComplete := v.populate(ctx, block)
	return isComplete
}

func (v *TimeValidityWindow[T]) Accept(blk ExecutionBlock[T]) {
	// Grab the mu before modifiying seen
	v.mu.Lock()
	defer v.mu.Unlock()

	evicted := v.seen.SetMin(blk.GetTimestamp())
	v.log.Debug("accepting block to validity window",
		zap.Stringer("blkID", blk.GetID()),
		zap.Time("minTimestamp", time.UnixMilli(blk.GetTimestamp())),
		zap.Int("evicted", len(evicted)),
	)
	v.seen.Add(blk.GetContainers())
	v.lastAcceptedBlockHeight = blk.GetHeight()
}

func (v *TimeValidityWindow[T]) AcceptHistorical(blk ExecutionBlock[T]) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.log.Debug("adding historical block to validity window",
		zap.Stringer("blkID", blk.GetID()),
		zap.Uint64("height", blk.GetHeight()),
		zap.Time("timestamp", time.UnixMilli(blk.GetTimestamp())),
	)
	v.seen.Add(blk.GetContainers())
}

func (v *TimeValidityWindow[T]) VerifyExpiryReplayProtection(
	ctx context.Context,
	blk ExecutionBlock[T],
) error {
	_, span := v.tracer.Start(ctx, "Chain.VerifyExpiryReplayProtection")
	defer span.End()

	v.mu.Lock()
	lastAcceptedBlockHeight := v.lastAcceptedBlockHeight
	v.mu.Unlock()

	if blk.GetHeight() <= lastAcceptedBlockHeight {
		return nil
	}

	// make sure we have no repeats within the block itself.
	blkContainerIDs := set.NewSet[ids.ID](len(blk.GetContainers()))
	for _, container := range blk.GetContainers() {
		containerID := container.GetID()
		if blkContainerIDs.Contains(containerID) {
			return fmt.Errorf("%w: %s", ErrDuplicateContainer, containerID)
		}
		blkContainerIDs.Add(containerID)
	}

	parent, err := v.chainIndex.GetExecutionBlock(ctx, blk.GetParent())
	if err != nil {
		return err
	}

	oldestAllowed := v.calculateOldestAllowed(blk.GetTimestamp())
	dup, err := v.isRepeat(ctx, parent, oldestAllowed, blk.GetContainers(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: contains %d duplicates out of %d containers", ErrDuplicateContainer, dup.BitLen(), len(blk.GetContainers()))
	}
	return nil
}

func (v *TimeValidityWindow[T]) IsRepeat(
	ctx context.Context,
	parentBlk ExecutionBlock[T],
	currentTimestamp int64,
	containers []T,
) (set.Bits, error) {
	_, span := v.tracer.Start(ctx, "Chain.IsRepeat")
	defer span.End()
	oldestAllowed := v.calculateOldestAllowed(currentTimestamp)
	return v.isRepeat(ctx, parentBlk, oldestAllowed, containers, false)
}

func (v *TimeValidityWindow[T]) isRepeat(
	ctx context.Context,
	ancestorBlk ExecutionBlock[T],
	oldestAllowed int64,
	containers []T,
	stop bool,
) (set.Bits, error) {
	marker := set.NewBits()

	v.mu.Lock()
	defer v.mu.Unlock()

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

func (v *TimeValidityWindow[T]) populate(ctx context.Context, block ExecutionBlock[T]) ([]ExecutionBlock[T], bool) {
	var (
		parent             = block
		parents            = []ExecutionBlock[T]{parent}
		fullValidityWindow = false
		oldestAllowed      = v.calculateOldestAllowed(block.GetTimestamp())
		err                error
	)

	// Keep fetching parents until we:
	// - Reach block height 0 (Genesis) at that point we have a full validity window,
	// and we can correctly preform replay protection
	// - Fill a validity window, or
	// - Can't find more blocks
	// Descending order is guaranteed by the parent-based traversal method
	for {
		if parent.GetHeight() == 0 {
			fullValidityWindow = true
			break
		}

		// Get execution block from cache or disk
		parent, err = v.chainIndex.GetExecutionBlock(ctx, parent.GetParent())
		if err != nil {
			break // This is expected when we run out-of-cached and/or on-disk blocks
		}
		parents = append(parents, parent)

		fullValidityWindow = parent.GetTimestamp() < oldestAllowed
		if fullValidityWindow {
			break
		}
	}

	// Reverse blocks to process in chronological order
	slices.Reverse(parents)
	for _, blk := range parents {
		v.Accept(blk)
	}

	return parents, fullValidityWindow
}

func (v *TimeValidityWindow[T]) calculateOldestAllowed(timestamp int64) int64 {
	return max(0, timestamp-v.getTimeValidityWindow(timestamp))
}

func VerifyTimestamp(containerTimestamp int64, executionTimestamp int64, divisor int64, validityWindow int64) error {
	switch {
	case containerTimestamp%divisor != 0:
		return fmt.Errorf("%w: timestamp (%d) %% divisor (%d) != 0", ErrMisalignedTime, containerTimestamp, divisor)
	case containerTimestamp < executionTimestamp: // expiry: 100 block: 110
		return fmt.Errorf("%w: timestamp (%d) < block timestamp (%d)", ErrTimestampExpired, containerTimestamp, executionTimestamp)
	case containerTimestamp > executionTimestamp+validityWindow: // expiry: 100 block 10
		return fmt.Errorf("%w: timestamp (%d) > block timestamp (%d) + validity window (%d)", ErrFutureTimestamp, containerTimestamp, executionTimestamp, validityWindow)
	default:
		return nil
	}
}
