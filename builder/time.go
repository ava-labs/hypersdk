// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/AnomalyFi/hypersdk/window"
	"go.uber.org/zap"
)

const buildCheck = 50 * time.Millisecond

var _ Builder = (*Time)(nil)

// Time tells the engine when to build blocks and gossip transactions
type Time struct {
	vm  VM
	cfg *TimeConfig

	doneBuild chan struct{}

	lastBuild time.Time
}

type TimeConfig struct {
	PreferredBlocksPerSecond uint64
	BuildInterval            time.Duration
}

func DefaultTimeConfig() *TimeConfig {
	return &TimeConfig{
		PreferredBlocksPerSecond: 2, // make sure to synchronize this with any VMs
		BuildInterval:            200 * time.Millisecond,
	}
}

func NewTime(vm VM, cfg *TimeConfig) *Time {
	b := &Time{
		vm:  vm,
		cfg: cfg,

		doneBuild: make(chan struct{}),
	}
	return b
}

// HandleGenerateBlock should be called immediately after [BuildBlock].
// [HandleGenerateBlock] invocation could lead to quiesence, building a block with
// some delay, or attempting to build another block immediately.
func (b *Time) HandleGenerateBlock() {
	b.lastBuild = time.Now()
}

func (b *Time) shouldBuild(ctx context.Context) (bool, error) {
	preferredBlk, err := b.vm.PreferredBlock(ctx)
	if err != nil {
		return false, err
	}
	now := time.Now().Unix()
	since := int(now - preferredBlk.Tmstmp)
	newRollupWindow, err := window.Roll(preferredBlk.BlockWindow, since)
	if err != nil {
		return false, err
	}
	// [preferredBlk.BlockWindow] already includes the preferred block, so we
	// don't need to add 1 before determining if we should build another block.
	return window.Last(&newRollupWindow) < b.cfg.PreferredBlocksPerSecond, nil
}

func (b *Time) Run() {
	b.vm.Logger().Info(
		"starting builder",
		zap.Duration("interval", b.cfg.BuildInterval),
	)
	defer close(b.doneBuild)

	t := time.NewTicker(buildCheck)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			ctx := context.Background()

			// Prevent runaway block production during window
			if time.Since(b.lastBuild) < b.cfg.BuildInterval {
				b.vm.Logger().Debug("skipping build because build block recently")
				continue
			}
			if b.vm.Mempool().Len(ctx) == 0 {
				b.vm.Logger().Debug("skipping build because no transactions in mempool")
				continue
			}
			build, err := b.shouldBuild(ctx)
			if err != nil {
				b.vm.Logger().Warn(
					"unable to determined if should build, building anyways",
					zap.Error(err),
				)
			}
			if !build {
				continue
			}
			b.TriggerBuild()
		case <-b.vm.StopChan():
			b.vm.Logger().Info("stopping build loop")
			return
		}
	}
}

func (b *Time) TriggerBuild() {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
}

func (b *Time) Done() {
	<-b.doneBuild
}
