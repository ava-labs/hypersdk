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

	"github.com/ava-labs/hypersdk/chain"
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

// Time tells the engine when to build blocks and gossip transactions
type Time struct {
	engineCh    chan<- common.Message
	logger      logging.Logger
	mempool     Mempool
	ruleFactory chain.RuleFactory
	doneBuild   chan struct{}

	timer     *timer.Timer
	lastQueue int64
	waiting   atomic.Bool
}

func NewTime(engineCh chan<- common.Message, logger logging.Logger, mempool Mempool, ruleFactory chain.RuleFactory) *Time {
	b := &Time{
		engineCh:    engineCh,
		logger:      logger,
		mempool:     mempool,
		ruleFactory: ruleFactory,
		doneBuild:   make(chan struct{}),
	}
	b.timer = timer.NewTimer(b.handleTimerNotify)
	return b
}

func (b *Time) Run(preferedBlkTmstmp int64) {
	b.Queue(context.TODO(), preferedBlkTmstmp) // start building loop (may not be an initial trigger)
	b.timer.Dispatch()                         // this blocks
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

func (b *Time) nextTime(now int64, preferred int64) int64 {
	gap := b.ruleFactory.GetRules(now).GetMinBlockGap()
	next := max(b.lastQueue+minBuildGap, preferred+gap)
	if next < now {
		return -1
	}
	return next
}

func (b *Time) Queue(ctx context.Context, preferedBlkTmstmp int64) {
	if !b.waiting.CompareAndSwap(false, true) {
		b.logger.Debug("unable to acquire waiting lock")
		return
	}
	now := time.Now().UnixMilli()
	next := b.nextTime(now, preferedBlkTmstmp)
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
