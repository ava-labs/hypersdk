// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/timer"
	"go.uber.org/zap"
)

// minBuildGap ensures we don't build blocks too quickly (can fail
// if we build empty blocks too soon)
//
// TODO: consider replacing this with AvalancheGo block build metering
const minBuildGap int64 = 25 // ms

var _ Builder = (*Time)(nil)

type Mempool interface {
	Len(context.Context) int // items
}

// GetPreferredTimestampAndBlockGap function accepts the current timestamp and returns
// the preferred block's timestamp and the current rule's block gap.
// If it fails to get the block, it should return a non-nil error.
type GetPreferredTimestampAndBlockGap func(ctx context.Context, now int64) (preferredTimestamp int64, blockGap int64, err error)

// Time tells the engine when to build blocks and gossip transactions
type Time struct {
	engineCh                         chan<- common.Message
	logger                           logging.Logger
	mempool                          Mempool
	getPreferredTimestampAndBlockGap GetPreferredTimestampAndBlockGap
	doneBuild                        chan struct{}
	cancelCtxFunc                    context.CancelFunc

	timer     *timer.Timer
	lastQueue int64
	waiting   atomic.Bool
}

func NewTime(engineCh chan<- common.Message, logger logging.Logger, mempool Mempool, getPreferredTimestampAndBlockGap GetPreferredTimestampAndBlockGap) *Time {
	cancelCtx, cancelCtxFunc := context.WithCancel(context.Background())
	b := &Time{
		engineCh:                         engineCh,
		logger:                           logger,
		mempool:                          mempool,
		getPreferredTimestampAndBlockGap: getPreferredTimestampAndBlockGap,
		doneBuild:                        make(chan struct{}),
		cancelCtxFunc:                    cancelCtxFunc,
	}
	b.timer = timer.NewTimer(func() {
		b.handleTimerNotify(cancelCtx)
	})

	return b
}

func (b *Time) Start() {
	b.Queue(context.TODO()) // start building loop (may not be an initial trigger)
	go b.timer.Dispatch()   // this blocks
}

func (b *Time) handleTimerNotify(ctx context.Context) {
	if err := b.Force(ctx); err != nil {
		b.logger.Warn("unable to build", zap.Error(err))
	} else {
		txs := b.mempool.Len(ctx)
		b.logger.Debug("trigger to notify", zap.Int("txs", txs))
	}
	b.waiting.Store(false)
}

func (b *Time) nextTime(now, preferred, gap int64) int64 {
	next := max(b.lastQueue+minBuildGap, preferred+gap)
	if next < now {
		return -1
	}
	return next
}

func (b *Time) Queue(ctx context.Context) {
	if !b.waiting.CompareAndSwap(false, true) {
		b.logger.Debug("unable to acquire waiting lock")
		return
	}
	now := time.Now().UnixMilli()
	preferred, gap, err := b.getPreferredTimestampAndBlockGap(ctx, now)
	if err != nil {
		// unable to retrieve block.
		b.waiting.Store(false)
		b.logger.Warn("unable to get preferred timestamp and block gap", zap.Error(err))
		return
	}
	next := b.nextTime(now, preferred, gap)
	if next < 0 {
		if err := b.Force(ctx); err != nil {
			b.logger.Warn("unable to build", zap.Error(err))
		} else {
			txs := b.mempool.Len(ctx)
			b.logger.Debug("notifying to build without waiting", zap.Int("txs", txs))
		}
		b.waiting.Store(false)
		return
	}
	sleep := next - now
	sleepDur := time.Duration(sleep * int64(time.Millisecond))
	b.timer.SetTimeoutIn(sleepDur)
	b.logger.Debug("waiting to notify to build", zap.Duration("t", sleepDur))
}

func (b *Time) Force(context.Context) error {
	select {
	case b.engineCh <- common.PendingTxs:
		b.lastQueue = time.Now().UnixMilli()
	default:
		b.logger.Debug("dropping message to consensus engine")
	}
	return nil
}

func (b *Time) Done() {
	b.cancelCtxFunc()
	b.timer.Stop()
}
