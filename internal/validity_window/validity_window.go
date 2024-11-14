// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validity_window

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/emap"
)

var ErrDuplicateTx = errors.New("duplicate transaction")

type timeValidityWindow[TxnTypePtr emap.Item] struct {
	log    logging.Logger
	tracer trace.Tracer

	lock       sync.Mutex
	chainIndex ChainIndex[TxnTypePtr]
	seen       *emap.EMap[TxnTypePtr]
}

func NewTimeValidityWindow[TxnTypePtr emap.Item](log logging.Logger, tracer trace.Tracer, chainIndex ChainIndex[TxnTypePtr]) TimeValidityWindow[TxnTypePtr] {
	return &timeValidityWindow[TxnTypePtr]{
		log:        log,
		tracer:     tracer,
		chainIndex: chainIndex,
		seen:       emap.NewEMap[TxnTypePtr](),
	}
}

func (v *timeValidityWindow[TxnTypePtr]) Accept(blk ExecutionBlock[TxnTypePtr]) {
	// Grab the lock before modifiying seen
	v.lock.Lock()
	defer v.lock.Unlock()

	evicted := v.seen.SetMin(blk.Timestamp())
	v.log.Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	v.seen.Add(blk.Txs())
}

func (v *timeValidityWindow[TxnTypePtr]) VerifyExpiryReplayProtection(
	ctx context.Context,
	blk ExecutionBlock[TxnTypePtr],
	oldestAllowed int64,
) error {
	lastAcceptedBlkHeight := v.chainIndex.LastAcceptedBlockHeight()
	if blk.Height() <= lastAcceptedBlkHeight {
		return nil
	}
	parent, err := v.chainIndex.GetExecutionBlock(ctx, blk.Parent())
	if err != nil {
		return err
	}

	dup, err := v.isRepeat(ctx, parent, oldestAllowed, blk.Txs(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}
	return nil
}

func (v *timeValidityWindow[TxnTypePtr]) IsRepeat(
	ctx context.Context,
	parentBlk ExecutionBlock[TxnTypePtr],
	txs []TxnTypePtr,
	oldestAllowed int64,
) (set.Bits, error) {
	return v.isRepeat(ctx, parentBlk, oldestAllowed, txs, false)
}

func (v *timeValidityWindow[TxnTypePtr]) isRepeat(
	ctx context.Context,
	ancestorBlk ExecutionBlock[TxnTypePtr],
	oldestAllowed int64,
	txs []TxnTypePtr,
	stop bool,
) (set.Bits, error) {
	marker := set.NewBits()

	_, span := v.tracer.Start(ctx, "Chain.isRepeat")
	defer span.End()

	v.lock.Lock()
	defer v.lock.Unlock()

	lastAcceptedBlkHeight := v.chainIndex.LastAcceptedBlockHeight()

	var err error
	for {
		if ancestorBlk.Timestamp() < oldestAllowed {
			return marker, nil
		}

		if ancestorBlk.Height() <= lastAcceptedBlkHeight || ancestorBlk.Height() == 0 {
			return v.seen.Contains(txs, marker, stop), nil
		}

		for i, tx := range txs {
			if marker.Contains(i) {
				continue
			}
			if err := ancestorBlk.InitTxs(); err != nil {
				return marker, err
			}
			if ancestorBlk.ContainsTx(tx.ID()) {
				marker.Add(i)
				if stop {
					return marker, nil
				}
			}
		}

		ancestorBlk, err = v.chainIndex.GetExecutionBlock(ctx, ancestorBlk.Parent())
		if err != nil {
			return marker, err
		}
	}
}
