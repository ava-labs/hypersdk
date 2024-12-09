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

var _ snowman.Block = (*StatefulBlock[Block, Block, Block])(nil)

type Block interface {
	fmt.Stringer
	ID() ids.ID
	Parent() ids.ID
	Timestamp() int64
	Bytes() []byte
	GetStateRoot() ids.ID
	Height() uint64
}

// StatefulBlock implements snowman.Block and abstracts away the caching
// and block pinning required by the AvalancheGo Consensus engine. This
// converts from the AvalancheGo VM block.ChainVM and snowman.Block interfaces
// to implementing a stateless block interface and each state transition with
// all of the inputs that the chain could depend on.
// 1. Verify a block against a verified parent
// 2. Accept a verified block
// 3. Reject a verified block
type StatefulBlock[I Block, O Block, A Block] struct {
	Input    I
	Output   O
	verified bool
	Accepted A
	accepted bool

	vm *CovariantVM[I, O, A]
}

func NewAcceptedBlock[I Block, O Block, A Block](
	vm *CovariantVM[I, O, A],
	input I,
	output O,
	accepted A,
) *StatefulBlock[I, O, A] {
	return &StatefulBlock[I, O, A]{
		Input:    input,
		Output:   output,
		verified: true,
		Accepted: accepted,
		accepted: true,
		vm:       vm,
	}
}

func NewVerifiedBlock[I Block, O Block, A Block](
	vm *CovariantVM[I, O, A],
	input I,
	output O,
) *StatefulBlock[I, O, A] {
	return &StatefulBlock[I, O, A]{
		Input:    input,
		Output:   output,
		verified: true,
		vm:       vm,
	}
}

func NewInputBlock[I Block, O Block, A Block](
	vm *CovariantVM[I, O, A],
	input I,
) *StatefulBlock[I, O, A] {
	return &StatefulBlock[I, O, A]{
		Input: input,
		vm:    vm,
	}
}

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) Verify(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.metrics.blockVerify.Observe(float64(time.Since(start)))
	}()

	stateReady := b.vm.Options.StateSyncClient.StateReady()
	ctx, span := b.vm.tracer.Start(
		ctx, "StatefulBlock.Verify",
		trace.WithAttributes(
			attribute.Int("size", len(b.Input.Bytes())),
			attribute.Int64("height", int64(b.Input.Height())),
			attribute.Bool("stateReady", stateReady),
			attribute.Bool("built", b.verified),
		),
	)
	defer span.End()

	switch {
	case !stateReady:
		// If the state of the accepted tip has not been fully fetched, it is not safe to
		// verify any block.
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
		// Parent block may not be processed when we verify this block, so [innerVerify] may
		// recursively verify ancestry.
		if err := b.innerVerify(ctx); err != nil {
			b.vm.log.Warn("verification failed",
				zap.Uint64("height", b.Input.Height()),
				zap.Stringer("blkID", b.Input.ID()),
				zap.Error(err),
			)
			return err
		}
	}

	b.vm.verifiedL.Lock()
	b.vm.verifiedBlocks[b.Input.ID()] = b
	b.vm.verifiedL.Unlock()

	if err := event.NotifyAll[O](ctx, b.Output, b.vm.Options.VerifiedSubs...); err != nil {
		return err
	}

	if b.verified {
		b.vm.log.Debug("verified block",
			zap.Stringer("blk", b.Output),
			zap.Bool("stateReady", stateReady),
		)
	} else {
		b.vm.log.Debug("skipped block verification",
			zap.Stringer("blk", b.Input),
			zap.Bool("stateReady", stateReady),
		)
	}
	return nil
}

// innerVerify executes the block
//
// Invariants:
// Accepted / Rejected blocks should never have Verify called on them.
// Blocks that were verified (and returned nil) with Verify will not have verify called again.
//
// When this may be called:
//  1. Verify
//  2. If the parent view is missing when verifying (dynamic state sync)
//  3. If the view of a block we are accepting is missing (finishing dynamic
//     state sync)
func (b *StatefulBlock[I, O, A]) innerVerify(ctx context.Context) error {
	parentBlk, err := b.vm.GetBlock(ctx, b.Parent())
	if err != nil {
		return fmt.Errorf("failed to fetch parent block: %w", err)
	}
	output, err := b.vm.chain.Execute(ctx, parentBlk.Output, b.Input)
	if err != nil {
		return err
	}
	b.Output = output
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O, A]) Accept(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.metrics.blockAccept.Observe(float64(time.Since(start)))
	}()

	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Accept")
	defer span.End()

	// Consider verifying the a block if it is not processed and we are no longer
	// syncing.
	if !b.verified {
		// The state of this block was not calculated during the call to
		// [StatefulBlock.Verify]. This is because the VM was state syncing
		// and did not have the state necessary to verify the block.
		// TODO: Move dynamic state sync block wrapper into state sync package.
		updated, err := b.vm.Options.StateSyncClient.UpdateSyncTarget(b)
		if err != nil {
			return err
		}
		if updated {
			b.vm.log.Info("updated state sync target",
				zap.Stringer("id", b.Input.ID()),
				zap.Stringer("root", b.Input.GetStateRoot()),
			)
			return nil // the sync is still ongoing
		}

		// This code handles the case where this block was not
		// verified during state sync (stopped syncing with a
		// processing block).
		//
		// If state sync completes before accept is called
		// then we need to process it here.
		b.vm.log.Info("verifying unprocessed block in accept",
			zap.Stringer("id", b.Input.ID()),
			zap.Stringer("root", b.Input.GetStateRoot()),
		)
		if err := b.innerVerify(ctx); err != nil {
			return fmt.Errorf("%w: unable to verify block", err)
		}
	}

	acceptedBlk, err := b.vm.chain.AcceptBlock(ctx, b.Output)
	if err != nil {
		return err
	}
	b.Accepted = acceptedBlk
	b.accepted = true

	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.ID())
	b.vm.verifiedL.Unlock()

	// Mark block as accepted and update last accepted in storage
	return b.MarkAccepted(ctx)
}

// implements "statesync.StateSummaryBlock"
func (b *StatefulBlock[I, O, A]) MarkAccepted(ctx context.Context) error {
	// Accept block and free unnecessary memory
	b.accepted = true

	// [Accepted] will persist the block to disk and set in-memory variables
	// needed to ensure we don't resync all blocks when state sync finishes.
	//
	// Note: We will not call [b.vm.Verified] before accepting during state sync
	return event.NotifyAll[A](ctx, b.Accepted, b.vm.Options.AcceptedSubs...)
}

// implements "statesync.StateSummaryBlock"
func (b *StatefulBlock[I, O, A]) GetStateRoot() ids.ID { return b.Input.GetStateRoot() }

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O, A]) Reject(ctx context.Context) error {
	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Reject")
	defer span.End()

	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.ID())
	b.vm.verifiedL.Unlock()

	return event.NotifyAll[O](ctx, b.Output, b.vm.Options.RejectedSubs...)
}

// Testing
func (b *StatefulBlock[I, O, A]) MarkUnprocessed() {
	var (
		emptyOutput   O
		emptyAccepted A
	)
	b.Output = emptyOutput
	b.verified = false
	b.Accepted = emptyAccepted
	b.accepted = false
}

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) Parent() ids.ID { return b.Input.ID() }

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) Height() uint64 { return b.Input.Height() }

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) Timestamp() time.Time { return time.UnixMilli(b.Input.Timestamp()) }

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) Bytes() []byte { return b.Input.Bytes() }

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) ID() ids.ID { return b.Input.ID() }

// implements "fmt.Stringer"
func (b *StatefulBlock[I, O, A]) String() string { return b.Input.String() }
