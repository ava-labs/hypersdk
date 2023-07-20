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

var _ Builder = (*Time)(nil)

// Time tells the engine when to build blocks and gossip transactions
type Time struct {
	vm        VM
	doneBuild chan struct{}

	timer   *timer.Timer
	waiting atomic.Bool
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
	b.timer.Dispatch()
}

func (b *Time) handleTimerNotify() {
	// Avoid notifying the engine if there is nothing to do (will block when VM
	// is later invoked)
	var sent bool
	if b.vm.Mempool().Len(context.TODO()) > 0 {
		b.ForceNotify()
		sent = true
	}
	b.waiting.Store(false)
	b.vm.Logger().Debug("trigger to notify", zap.Bool("sent", sent))
}

func (b *Time) QueueNotify() {
	if !b.waiting.CompareAndSwap(false, true) {
		return
	}
	preferredBlk, err := b.vm.PreferredBlock(context.TODO())
	if err != nil {
		b.waiting.Store(false)
		b.vm.Logger().Warn("unable to load preferred block", zap.Error(err))
		return
	}
	now := time.Now().UnixMilli()
	since := now - preferredBlk.Tmstmp
	if since < 0 {
		since = 0
	}
	gap := b.vm.Rules(now).GetMinBlockGap()
	if since >= gap {
		var sent bool
		if b.vm.Mempool().Len(context.TODO()) > 0 {
			b.ForceNotify()
			sent = true
		}
		b.waiting.Store(false)
		b.vm.Logger().Debug("notifying to build without waiting", zap.Bool("sent", sent))
		return
	}
	sleep := gap - since
	sleepDur := time.Duration(sleep * int64(time.Millisecond))
	b.timer.SetTimeoutIn(sleepDur)
	b.vm.Logger().Debug("waiting to notify to build", zap.Duration("t", sleepDur))
}

func (b *Time) ForceNotify() {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
}

func (b *Time) Done() {
	b.timer.Stop()
}
