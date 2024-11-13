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

type Mempool interface {
	Len(context.Context) int // items
}

type GetPreferedTimestampAndBlockGap func(int64) (int64, int64)

// Time tells the engine when to build blocks and gossip transactions
type Time struct {
	engineCh                        chan<- common.Message
	logger                          logging.Logger
	mempool                         Mempool
	getPreferedTimestampAndBlockGap GetPreferedTimestampAndBlockGap
	doneBuild                       chan struct{}

	timer     *timer.Timer
	lastQueue int64
	waiting   atomic.Bool
}

func NewTime(engineCh chan<- common.Message, logger logging.Logger, mempool Mempool, getPreferedTimestampAndBlockGap GetPreferedTimestampAndBlockGap) *Time {
	b := &Time{
		engineCh:                        engineCh,
		logger:                          logger,
		mempool:                         mempool,
		getPreferedTimestampAndBlockGap: getPreferedTimestampAndBlockGap,
		doneBuild:                       make(chan struct{}),
	}
	b.timer = timer.NewTimer(b.handleTimerNotify)
	return b
}

func (b *Time) Run() {
	b.Queue(context.TODO()) // start building loop (may not be an initial trigger)
	b.timer.Dispatch()      // this blocks
}

func (b *Time) handleTimerNotify() {
	if err := b.Force(context.TODO()); err != nil {
		b.logger.Warn("unable to build", zap.Error(err))
	} else {
		txs := b.mempool.Len(context.TODO())
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
	preferred, gap := b.getPreferedTimestampAndBlockGap(now)
	if gap < 0 {
		// unable to retrieve block.
		b.logger.Warn("unable to get preferred timestamp and block gap")
		return
	}
	next := b.nextTime(now, preferred, gap)
	if next < 0 {
		if err := b.Force(ctx); err != nil {
			b.logger.Warn("unable to build", zap.Error(err))
		} else {
			txs := b.mempool.Len(context.TODO())
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
	b.timer.Stop()
}
