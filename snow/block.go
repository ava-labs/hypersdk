// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"
	"slices"
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

	ready := b.vm.Options.Ready.Ready()
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
		parent, err := b.vm.GetBlock(ctx, b.Parent())
		if err != nil {
			return err
		}
		// If the parent has not been verified and we're no longer in dynamic state sync,
		// attempt to verify from the last accepted block through to the parent.
		// Any error along this chain will invalidate the entire chain.
		if !parent.verified {
			blksToProcess, err := b.getAncestorsToLastAccepted(ctx)
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

		if err := event.NotifyAll[O](ctx, b.Output, b.vm.Options.VerifiedSubs...); err != nil {
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

// verify the block against the provided parent output and set the
// required Output/verified fields.
func (b *StatefulBlock[I, O, A]) verify(ctx context.Context, parentOutput O) error {
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
func (b *StatefulBlock[I, O, A]) accept(ctx context.Context) error {
	acceptedBlk, err := b.vm.chain.AcceptBlock(ctx, b.Output)
	if err != nil {
		return err
	}
	b.Accepted = acceptedBlk
	b.accepted = true
	return nil
}

// markAccepted marks the block and updates the required VM state.
func (b *StatefulBlock[I, O, A]) markAccepted(ctx context.Context) error {
	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.ID())
	b.vm.verifiedL.Unlock()
	b.vm.covariantVM.lastAcceptedBlock = b

	if b.accepted {
		return event.NotifyAll(ctx, b.Accepted, b.vm.Options.AcceptedSubs...)
	} else {
		return event.NotifyAll(ctx, b.Input, b.vm.Options.PreReadyAcceptedSubs...)
	}
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O, A]) Accept(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.metrics.blockAccept.Observe(float64(time.Since(start)))
	}()

	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Accept")
	defer span.End()

	// If I've already been verified, accept myself.
	if b.verified {
		if err := b.accept(ctx); err != nil {
			return err
		}
		return b.markAccepted(ctx)
	}

	// If I'm not ready yet, mark myself as accepted, and return early.
	isReady := b.vm.Options.Ready.Ready()
	if !isReady {
		return b.markAccepted(ctx)
	}

	// If I haven't verified myself, then I need to verify myself before before
	// accepting myself.
	// My parent must either be directly verified or have been fully populated
	// by state sync completing.
	parent, err := b.vm.GetBlock(ctx, b.Parent())
	if err != nil {
		return fmt.Errorf("failed to fetch parent while accepting %s: %w", b, err)
	}
	if err := b.verify(ctx, parent.Output); err != nil {
		return err
	}
	if err := b.accept(ctx); err != nil {
		return err
	}
	return b.markAccepted(ctx)
}

// implements "statesync.StateSummaryBlock"
func (b *StatefulBlock[I, O, A]) AcceptSyncTarget(ctx context.Context) error {
	return event.NotifyAll[I](ctx, b.Input, b.vm.Options.PreReadyAcceptedSubs...)
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

	if b.verified {
		return event.NotifyAll[O](ctx, b.Output, b.vm.Options.RejectedSubs...)
	}
	return nil
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

// Testing
func (b *StatefulBlock[I, O, A]) markProcessed(output O, accepted A) {
	b.Output = output
	b.verified = true
	b.Accepted = accepted
	b.accepted = true
}

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) Parent() ids.ID { return b.Input.Parent() }

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

// getAncestorsToLastAccepted returns the range of blocks (lastAccepted, b)
func (b *StatefulBlock[I, O, A]) getAncestorsToLastAccepted(ctx context.Context) ([]*StatefulBlock[I, O, A], error) {
	blksToProcess := make([]*StatefulBlock[I, O, A], 0)
	nextBlock := b
	for {
		parent, err := b.vm.GetBlock(ctx, nextBlock.Parent())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch parent block while verifying ancestors: %w", err)
		}
		if parent.ID() == b.vm.lastAcceptedBlock.ID() {
			break
		}
		if parent.Height() <= b.vm.lastAcceptedBlock.Height() {
			return nil, fmt.Errorf("hit unexpected block while verifying ancestors: %d <= %d for block %s", parent.Height(), b.vm.lastAcceptedBlock.Height(), b)
		}
		blksToProcess = append(blksToProcess, parent)
		nextBlock = parent
	}
	slices.Reverse(blksToProcess)
	return blksToProcess, nil
}
