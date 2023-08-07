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

const minBuildGap int64 = 30 // ms

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
	b.QueueNotify()    // start building loop (may not be an initial trigger)
	b.timer.Dispatch() // this blocks
}

func (b *Time) handleTimerNotify() {
	b.ForceNotify()
	b.waiting.Store(false)
	txs := b.vm.Mempool().Len(context.TODO())
	b.vm.Logger().Debug("trigger to notify", zap.Int("txs", txs))
}

func (b *Time) QueueNotify() {
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
	gap := b.vm.Rules(now).GetMinBlockGap()
	var sleep int64
	if now >= preferredBlk.Tmstmp+gap {
		if now >= b.lastQueue+minBuildGap {
			b.ForceNotify()
			b.waiting.Store(false)
			txs := b.vm.Mempool().Len(context.TODO())
			b.vm.Logger().Debug("notifying to build without waiting", zap.Int("txs", txs))
			return
		}
		sleep = b.lastQueue + minBuildGap - now
	} else {
		sleep = preferredBlk.Tmstmp + gap - now
	}
	sleepDur := time.Duration(sleep * int64(time.Millisecond))
	b.timer.SetTimeoutIn(sleepDur)
	b.vm.Logger().Debug("waiting to notify to build", zap.Duration("t", sleepDur))
}

func (b *Time) ForceNotify() {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
		b.lastQueue = time.Now().UnixMilli()
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
}

func (b *Time) Done() {
	b.timer.Stop()
}
