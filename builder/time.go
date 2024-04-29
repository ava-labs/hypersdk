// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/timer"
	"go.uber.org/zap"
)

// minBuildGap ensures we don't build blocks too quickly (can fail
// if we build empty blocks too soon)
//
// TODO: consider replacing this with AvalancheGo block build metering
const minBuildGap int64 = 25 // ms

var _ Builder = (*Time)(nil)

// Time tells the engine when to build blocks and gossip transactions
type Time struct {
	vm        VM
	doneBuild chan struct{}

	timer     *timer.Timer
	lastQueue int64
	waiting   atomic.Bool
}

func NewTime(vm VM) *Time {
	b := &Time{
		vm:        vm,
		doneBuild: make(chan struct{}),
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
		b.vm.Logger().Warn("unable to build", zap.Error(err))
	} else {
		txs := b.vm.Mempool().Len(context.TODO())
		b.vm.Logger().Debug("trigger to notify", zap.Int("txs", txs))
	}
	b.waiting.Store(false)
}

func (b *Time) nextTime(now int64, preferred int64) int64 {
	gap := b.vm.Rules(now).GetMinBlockGap()
	next := max(b.lastQueue+minBuildGap, preferred+gap)
	if next < now {
		return -1
	}
	return next
}

func (b *Time) Queue(ctx context.Context) {
	if !b.waiting.CompareAndSwap(false, true) {
		b.vm.Logger().Debug("unable to acquire waiting lock")
		return
	}
	preferredBlk, err := b.vm.PreferredBlock(context.TODO())
	if err != nil {
		b.waiting.Store(false)
		b.vm.Logger().Warn("unable to load preferred block", zap.Error(err))
		return
	}
	now := time.Now().UnixMilli()
	next := b.nextTime(now, preferredBlk.Tmstmp)
	if next < 0 {
		if err := b.Force(ctx); err != nil {
			b.vm.Logger().Warn("unable to build", zap.Error(err))
		} else {
			txs := b.vm.Mempool().Len(context.TODO())
			b.vm.Logger().Debug("notifying to build without waiting", zap.Int("txs", txs))
		}
		b.waiting.Store(false)
		return
	}
	sleep := next - now
	sleepDur := time.Duration(sleep * int64(time.Millisecond))
	b.timer.SetTimeoutIn(sleepDur)
	b.vm.Logger().Debug("waiting to notify to build", zap.Duration("t", sleepDur))
}

func (b *Time) Force(context.Context) error {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
		b.lastQueue = time.Now().UnixMilli()
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
	return nil
}

func (b *Time) Done() {
	b.timer.Stop()
}
