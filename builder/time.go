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
	b.ForceNotify()
	b.waiting.Store(false)
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
	wait := now - preferredBlk.Tmstmp + b.vm.Rules(now).GetMinBlockGap()
	if wait <= 0 {
		b.ForceNotify()
		b.waiting.Store(false)
		return
	}
	b.timer.SetTimeoutIn(time.Duration(wait * int64(time.Millisecond)))
}

func (b *Time) Notify() {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
}

func (b *Time) Done() {
	b.timer.Stop()
}
