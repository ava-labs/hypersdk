// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/internal/emap"
	"go.uber.org/zap"
)

type ChainIndex interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (*ExecutionBlock, error)
	LastAcceptedBlock() *ExecutionBlock
}

type TimeValidityWindowBackend interface {
	Logger() logging.Logger
	Tracer() trace.Tracer
	ChainIndex
}

type TimeValidityWindow struct {
	log    logging.Logger
	tracer trace.Tracer

	lock       sync.Mutex
	chainIndex ChainIndex
	seen       *emap.EMap[*Transaction]
}

func NewTimeValidityWindow(backend TimeValidityWindowBackend) *TimeValidityWindow {
	return &TimeValidityWindow{
		log:        backend.Logger(),
		tracer:     backend.Tracer(),
		chainIndex: backend,
		seen:       emap.NewEMap[*Transaction](),
	}
}

func (v *TimeValidityWindow) Accept(blk *ExecutionBlock) {
	// Grab the lock before modifiying seen
	v.lock.Lock()
	defer v.lock.Unlock()

	blkTime := blk.Tmstmp
	evicted := v.seen.SetMin(blkTime)
	v.log.Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	v.seen.Add(blk.Txs)
}

func (v *TimeValidityWindow) VerifyExpiryReplayProtection(
	ctx context.Context,
	blk *ExecutionBlock,
	oldestAllowed int64,
) error {
	lastAcceptedBlk := v.chainIndex.LastAcceptedBlock()
	if blk.Hght <= lastAcceptedBlk.Hght {
		return nil
	}
	parent, err := v.chainIndex.GetExecutionBlock(ctx, blk.Prnt)
	if err != nil {
		return err
	}

	dup, err := v.isRepeat(ctx, parent, oldestAllowed, blk.Txs, set.NewBits(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}
	return nil
}

func (v *TimeValidityWindow) IsRepeat(
	ctx context.Context,
	parentBlk *ExecutionBlock,
	timestamp int64,
	txs []*Transaction,
	oldestAllowed int64,
) (set.Bits, error) {
	_, span := v.tracer.Start(ctx, "chain.IsRepeat")
	defer span.End()

	return v.isRepeat(ctx, parentBlk, oldestAllowed, txs, set.NewBits(), false)
}

func (v *TimeValidityWindow) isRepeat(
	ctx context.Context,
	ancestorBlk *ExecutionBlock,
	oldestAllowed int64,
	txs []*Transaction,
	marker set.Bits,
	stop bool,
) (set.Bits, error) {
	_, span := v.tracer.Start(ctx, "chain.IsRepeat")
	defer span.End()

	v.lock.Lock()
	defer v.lock.Unlock()

	lastAcceptedBlk := v.chainIndex.LastAcceptedBlock()

	var err error
	for {
		if ancestorBlk.Tmstmp < oldestAllowed {
			return marker, nil
		}

		if ancestorBlk.Hght <= lastAcceptedBlk.Hght || ancestorBlk.Hght == 0 {
			return v.seen.Contains(txs, marker, stop), nil
		}

		for i, tx := range txs {
			if marker.Contains(i) {
				continue
			}
			if err := ancestorBlk.initTxs(); err != nil {
				return marker, err
			}
			if ancestorBlk.txsSet.Contains(tx.ID()) {
				marker.Add(i)
				if stop {
					return marker, nil
				}
			}
		}

		ancestorBlk, err = v.chainIndex.GetExecutionBlock(ctx, ancestorBlk.Prnt)
		if err != nil {
			return marker, err
		}
	}
}
