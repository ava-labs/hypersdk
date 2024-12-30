// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/event"
)

var _ snowman.Block = (*StatefulBlock[Block, Block])(nil)

type Block interface {
	fmt.Stringer
	ID() ids.ID
	Parent() ids.ID
	Timestamp() int64
	Bytes() []byte
	Height() uint64
}

// StatefulBlock implements snowman.Block and abstracts away the caching
// and block pinning required by the AvalancheGo Consensus engine.
// This converts the VM DevX from implementing the consensus engine specific invariants
// to implementing an input/output/accepted block type and handling the state transitions
// between these types.
// In conjunction with the AvalancheGo Consensus engine, this code guarantees that
// 1. Verify is always called against a verified parent
// 2. Accept is always called against a verified block
// 3. Reject is always called against a verified block
//
// StatefulBlock additionally handles DynamicStateSync where blocks are vaccuously
// verified/accepted to update a moving state sync target.
// After FinishStateSync is called, the snow package guarantees the same invariants
// as applied during normal consensus.
type StatefulBlock[I Block, O Block] struct {
	Input    I
	Output   O
	verified bool
	accepted bool

	vm *CovariantVM[I, O]
}

func NewInputBlock[I Block, O Block](
	vm *CovariantVM[I, O],
	input I,
) *StatefulBlock[I, O] {
	return &StatefulBlock[I, O]{
		Input: input,
		vm:    vm,
	}
}

func NewVerifiedBlock[I Block, O Block](
	vm *CovariantVM[I, O],
	input I,
	output O,
) *StatefulBlock[I, O] {
	return &StatefulBlock[I, O]{
		Input:    input,
		Output:   output,
		verified: true,
		vm:       vm,
	}
}

func NewAcceptedBlock[I Block, O Block](
	vm *CovariantVM[I, O],
	input I,
	output O,
) *StatefulBlock[I, O] {
	return &StatefulBlock[I, O]{
		Input:    input,
		Output:   output,
		verified: true,
		accepted: true,
		vm:       vm,
	}
}

func (b *StatefulBlock[I, O]) setAccepted(output O) {
	b.Output = output
	b.verified = true
	b.accepted = true
}

// verify the block against the provided parent output and set the
// required Output/verified fields.
func (b *StatefulBlock[I, O]) verify(ctx context.Context, parentOutput O) error {
	output, err := b.vm.chain.Execute(ctx, parentOutput, b.Input)
	if err != nil {
		return err
	}
	b.Output = output
	b.verified = true
	return nil
}

// accept the block and set the required Accepted/accepted fields.
// Assumes verify has already been called.
func (b *StatefulBlock[I, O]) accept(ctx context.Context, parentAccepted O) error {
	if err := b.vm.chain.AcceptBlock(ctx, parentAccepted, b.Output); err != nil {
		return err
	}
	b.accepted = true
	return nil
}

// implements "snowman.Block"
func (b *StatefulBlock[I, O]) Verify(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.metrics.blockVerify.Observe(float64(time.Since(start)))
	}()

	ready := b.vm.Ready()
	ctx, span := b.vm.tracer.Start(
		ctx, "StatefulBlock.Verify",
		trace.WithAttributes(
			attribute.Int("size", len(b.Input.Bytes())),
			attribute.Int64("height", int64(b.Input.Height())),
			attribute.Bool("ready", ready),
			attribute.Bool("built", b.verified),
		),
	)
	defer span.End()

	switch {
	case !ready:
		// If the VM is not ready (dynamic state sync), skip verifying the block.
		b.vm.log.Info(
			"skipping verification, state not ready",
			zap.Uint64("height", b.Input.Height()),
			zap.Stringer("blkID", b.Input.ID()),
		)
	case b.verified:
		// If we built the block, the state will already be populated and we don't
		// need to compute it (we assume that we built a correct block and it isn't
		// necessary to re-verify anything).
		b.vm.log.Info(
			"skipping verification of locally built block",
			zap.Uint64("height", b.Input.Height()),
			zap.Stringer("blkID", b.Input.ID()),
		)
	default:
		// Fetch my parent to verify against
		parent, err := b.vm.GetBlock(ctx, b.Parent())
		if err != nil {
			return err
		}

		// If my parent has not been verified and we're no longer in dynamic state sync,
		// we must be transitioning to normal consensus.
		// Attempt to verify from the last accepted block through to this block to
		// compute my parent's Output state.
		if !parent.verified {
			blksToProcess, err := b.vm.getExclusiveBlockRange(ctx, b.vm.lastAcceptedBlock, b)
			if err != nil {
				return err
			}
			parent := b.vm.lastAcceptedBlock
			for _, ancestor := range blksToProcess {
				if err := ancestor.verify(ctx, parent.Output); err != nil {
					return err
				}
				parent = ancestor
			}
		}

		// Verify the block against the parent
		if err := b.verify(ctx, parent.Output); err != nil {
			return err
		}

		if err := event.NotifyAll[O](ctx, b.Output, b.vm.app.VerifiedSubs...); err != nil {
			return err
		}
	}

	b.vm.verifiedL.Lock()
	b.vm.verifiedBlocks[b.Input.ID()] = b
	b.vm.verifiedL.Unlock()

	if b.verified {
		b.vm.log.Debug("verified block",
			zap.Stringer("blk", b.Output),
			zap.Bool("ready", ready),
		)
	} else {
		b.vm.log.Debug("skipped block verification",
			zap.Stringer("blk", b.Input),
			zap.Bool("ready", ready),
		)
	}
	return nil
}

// markAccepted marks the block and updates the required VM state.
func (b *StatefulBlock[I, O]) markAccepted(ctx context.Context) error {
	if err := b.vm.inputChainIndex.UpdateLastAccepted(ctx, b.Input); err != nil {
		return err
	}

	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.ID())
	b.vm.verifiedL.Unlock()
	b.vm.covariantVM.lastAcceptedBlock = b

	b.vm.acceptedBlocksByHeight.Put(b.Height(), b.ID())
	b.vm.acceptedBlocksByID.Put(b.ID(), b)

	return nil
}

func (b *StatefulBlock[I, O]) notifyAccepted(ctx context.Context) error {
	// If I was not actually marked accepted, notify pre ready subs
	if !b.accepted {
		return event.NotifyAll(ctx, b.Input, b.vm.app.PreReadyAcceptedSubs...)
	}

	return event.NotifyAll(ctx, b.Output, b.vm.app.AcceptedSubs...)
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O]) Accept(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.metrics.blockAccept.Observe(float64(time.Since(start)))
	}()

	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Accept")
	defer span.End()

	// If I've already been verified, accept myself.
	if b.verified {
		parent, err := b.vm.GetBlock(ctx, b.Parent())
		if err != nil {
			return fmt.Errorf("failed to fetch parent while accepting verified block %s: %w", b, err)
		}
		if err := b.markAccepted(ctx); err != nil {
			return err
		}
		if err := b.accept(ctx, parent.Output); err != nil {
			return err
		}
		return b.notifyAccepted(ctx)
	}

	// If I'm not ready yet, mark myself as accepted, and return early.
	isReady := b.vm.Ready()
	if !isReady {
		if err := b.markAccepted(ctx); err != nil {
			return err
		}
		return b.notifyAccepted(ctx)
	}

	// If I haven't verified myself, then I need to verify myself before
	// accepting myself.
	// if I am ready and haven't been verified, then I need to verify myself.
	// Note: I don't need to verify/accept my parent because my parent was already
	// marked as accepted and I'm ready. This means the last accepted block must
	// be fully populated.
	parent, err := b.vm.GetBlock(ctx, b.Parent())
	if err != nil {
		return fmt.Errorf("failed to fetch parent while accepting previously unverified block %s: %w", b, err)
	}
	if err := b.verify(ctx, parent.Output); err != nil {
		return err
	}
	if err := b.markAccepted(ctx); err != nil {
		return err
	}
	if err := b.accept(ctx, parent.Output); err != nil {
		return err
	}
	return b.notifyAccepted(ctx)
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O]) Reject(ctx context.Context) error {
	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Reject")
	defer span.End()

	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.ID())
	b.vm.verifiedL.Unlock()

	// Skip notifying rejected subs if we were still in dynamic state sync
	if !b.verified {
		return nil
	}

	return event.NotifyAll[O](ctx, b.Output, b.vm.app.RejectedSubs...)
}

// implements "snowman.Block"
func (b *StatefulBlock[I, O]) Parent() ids.ID { return b.Input.Parent() }

// implements "snowman.Block"
func (b *StatefulBlock[I, O]) Height() uint64 { return b.Input.Height() }

// implements "snowman.Block"
func (b *StatefulBlock[I, O]) Timestamp() time.Time { return time.UnixMilli(b.Input.Timestamp()) }

// implements "snowman.Block"
func (b *StatefulBlock[I, O]) Bytes() []byte { return b.Input.Bytes() }

// implements "snowman.Block"
func (b *StatefulBlock[I, O]) ID() ids.ID { return b.Input.ID() }

// implements "fmt.Stringer"
func (b *StatefulBlock[I, O]) String() string { return b.Input.String() }
