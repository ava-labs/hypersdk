// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/event"
)

var (
	_ snowman.Block           = (*StatefulBlock[Block, Block, Block])(nil)
	_ block.WithVerifyContext = (*StatefulBlock[Block, Block, Block])(nil)

	errParentFailedVerification = errors.New("parent failed verification")
	errMismatchedPChainContext  = errors.New("mismatched P-Chain context")
)

type Block interface {
	fmt.Stringer
	GetID() ids.ID
	GetParent() ids.ID
	GetTimestamp() int64
	GetBytes() []byte
	GetHeight() uint64
	// GetContext returns the P-Chain context of the block.
	// May return nil if there is no P-Chain context, which
	// should only occur prior to ProposerVM activation.
	// This will be verified from the snow package, so that the
	// inner chain can simply use its embedded context.
	GetContext() *block.Context
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
// StatefulBlock additionally handles DynamicStateSync where blocks are vacuously
// verified/accepted to update a moving state sync target.
// After FinishStateSync is called, the snow package guarantees the same invariants
// as applied during normal consensus.
type StatefulBlock[I Block, O Block, A Block] struct {
	Input    I
	Output   O
	verified bool
	Accepted A
	accepted bool

	vm *VM[I, O, A]
}

func NewInputBlock[I Block, O Block, A Block](
	vm *VM[I, O, A],
	input I,
) *StatefulBlock[I, O, A] {
	return &StatefulBlock[I, O, A]{
		Input: input,
		vm:    vm,
	}
}

func NewVerifiedBlock[I Block, O Block, A Block](
	vm *VM[I, O, A],
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

func NewAcceptedBlock[I Block, O Block, A Block](
	vm *VM[I, O, A],
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

func (b *StatefulBlock[I, O, A]) setAccepted(output O, accepted A) {
	b.Output = output
	b.verified = true
	b.Accepted = accepted
	b.accepted = true
}

// verify the block against the provided parent output and set the
// required Output/verified fields.
func (b *StatefulBlock[I, O, A]) verify(ctx context.Context, parentOutput O) error {
	output, err := b.vm.chain.VerifyBlock(ctx, parentOutput, b.Input)
	if err != nil {
		return err
	}
	b.Output = output
	b.verified = true
	return nil
}

// accept the block and set the required Accepted/accepted fields.
// Assumes verify has already been called.
func (b *StatefulBlock[I, O, A]) accept(ctx context.Context, parentAccepted A) error {
	acceptedBlk, err := b.vm.chain.AcceptBlock(ctx, parentAccepted, b.Output)
	if err != nil {
		return err
	}
	b.Accepted = acceptedBlk
	b.accepted = true
	return nil
}

func (*StatefulBlock[I, O, A]) ShouldVerifyWithContext(context.Context) (bool, error) {
	return true, nil
}

func (b *StatefulBlock[I, O, A]) VerifyWithContext(ctx context.Context, pChainCtx *block.Context) error {
	return b.verifyWithContext(ctx, pChainCtx)
}

func (b *StatefulBlock[I, O, A]) Verify(ctx context.Context) error {
	return b.verifyWithContext(ctx, nil)
}

func (b *StatefulBlock[I, O, A]) verifyWithContext(ctx context.Context, pChainCtx *block.Context) error {
	b.vm.chainLock.Lock()
	defer b.vm.chainLock.Unlock()

	start := time.Now()
	defer func() {
		b.vm.metrics.blockVerify.Observe(float64(time.Since(start)))
	}()

	ready := b.vm.ready
	ctx, span := b.vm.tracer.Start(
		ctx, "StatefulBlock.Verify",
		trace.WithAttributes(
			attribute.Int("size", len(b.Input.GetBytes())),
			attribute.Int64("height", int64(b.Input.GetHeight())),
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
			zap.Uint64("height", b.Input.GetHeight()),
			zap.Stringer("blkID", b.Input.GetID()),
		)
	case b.verified:
		// Defensive: verify the inner and wrapper block contexts match to ensure
		// we don't build a block with a mismatched P-Chain context that will be
		// invalid to peers.
		innerCtx := b.Input.GetContext()
		if err := verifyPChainCtx(pChainCtx, innerCtx); err != nil {
			return err
		}

		// If we built the block, the state will already be populated and we don't
		// need to compute it (we assume that we built a correct block and it isn't
		// necessary to re-verify).
		b.vm.log.Info(
			"skipping verification of locally built block",
			zap.Uint64("height", b.Input.GetHeight()),
			zap.Stringer("blkID", b.Input.GetID()),
		)
	default:
		b.vm.log.Info("Verifying block", zap.Stringer("block", b))
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
			return errParentFailedVerification
		} else {
			b.vm.log.Info("parent was already verified")
		}

		// Verify the inner and wrapper block contexts match
		innerCtx := b.Input.GetContext()
		if err := verifyPChainCtx(pChainCtx, innerCtx); err != nil {
			return err
		}
		if err := b.verify(ctx, parent.Output); err != nil {
			return err
		}

		if err := event.NotifyAll[O](ctx, b.Output, b.vm.verifiedSubs...); err != nil {
			return err
		}
	}

	b.vm.verifiedL.Lock()
	b.vm.verifiedBlocks[b.Input.GetID()] = b
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

func verifyPChainCtx(providedCtx, innerCtx *block.Context) error {
	switch {
	case providedCtx == nil && innerCtx == nil:
		return nil
	case providedCtx == nil && innerCtx != nil:
		return fmt.Errorf("%w: missing provided context != inner P-Chain height %d", errMismatchedPChainContext, innerCtx.PChainHeight)
	case providedCtx != nil && innerCtx == nil:
		return fmt.Errorf("%w: provided P-Chain height (%d) != missing inner context", errMismatchedPChainContext, providedCtx.PChainHeight)
	case providedCtx.PChainHeight != innerCtx.PChainHeight:
		return fmt.Errorf("%w: provided P-Chain height (%d) != inner P-Chain height %d", errMismatchedPChainContext, providedCtx.PChainHeight, innerCtx.PChainHeight)
	default:
		return nil
	}
}

// markAccepted marks the block and updates the required VM state.
// iff parent is non-nil, it will request the chain to Accept the block.
// The caller is responsible to provide the accepted parent if the VM is in a ready state.
func (b *StatefulBlock[I, O, A]) markAccepted(ctx context.Context, parent *StatefulBlock[I, O, A]) error {
	if err := b.vm.inputChainIndex.UpdateLastAccepted(ctx, b.Input); err != nil {
		return err
	}

	if parent != nil {
		if err := b.accept(ctx, parent.Accepted); err != nil {
			return err
		}
	}

	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.GetID())
	b.vm.verifiedL.Unlock()

	b.vm.setLastAccepted(b)

	return b.notifyAccepted(ctx)
}

func (b *StatefulBlock[I, O, A]) notifyAccepted(ctx context.Context) error {
	// If I was not actually marked accepted, notify pre ready subs
	if !b.accepted {
		return event.NotifyAll(ctx, b.Input, b.vm.preReadyAcceptedSubs...)
	}

	return event.NotifyAll(ctx, b.Accepted, b.vm.acceptedSubs...)
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O, A]) Accept(ctx context.Context) error {
	b.vm.chainLock.Lock()
	defer b.vm.chainLock.Unlock()

	start := time.Now()
	defer func() {
		b.vm.metrics.blockAccept.Observe(float64(time.Since(start)))
	}()

	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Accept")
	defer span.End()

	defer b.vm.log.Info("accepting block", zap.Stringer("block", b))

	// If I'm not ready yet, mark myself as accepted, and return early.
	isReady := b.vm.ready
	if !isReady {
		return b.markAccepted(ctx, nil)
	}

	// If I'm ready and not verified, then I or my ancestor must have failed
	// verification after completing dynamic state sync. This indicates
	// an invalid block has been accepted, which should be prevented by consensus.
	// If we hit this case, return a fatal error here.
	if !b.verified {
		return errParentFailedVerification
	}

	// If I am verified and ready, fetch my parent and accept myself. I'm verified, which
	// implies my parent is verified as well.
	parent, err := b.vm.GetBlock(ctx, b.Parent())
	if err != nil {
		return fmt.Errorf("failed to fetch parent while accepting verified block %s: %w", b, err)
	}
	return b.markAccepted(ctx, parent)
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock[I, O, A]) Reject(ctx context.Context) error {
	ctx, span := b.vm.tracer.Start(ctx, "StatefulBlock.Reject")
	defer span.End()

	b.vm.verifiedL.Lock()
	delete(b.vm.verifiedBlocks, b.Input.GetID())
	b.vm.verifiedL.Unlock()

	// Notify subscribers about the rejected blocks that were vacuously verified during dynamic state sync
	if !b.verified {
		return event.NotifyAll[I](ctx, b.Input, b.vm.preRejectedSubs...)
	}

	return event.NotifyAll[O](ctx, b.Output, b.vm.rejectedSubs...)
}

// implements "snowman.Block"
func (b *StatefulBlock[I, O, A]) ID() ids.ID           { return b.Input.GetID() }
func (b *StatefulBlock[I, O, A]) Parent() ids.ID       { return b.Input.GetParent() }
func (b *StatefulBlock[I, O, A]) Height() uint64       { return b.Input.GetHeight() }
func (b *StatefulBlock[I, O, A]) Timestamp() time.Time { return time.UnixMilli(b.Input.GetTimestamp()) }
func (b *StatefulBlock[I, O, A]) Bytes() []byte        { return b.Input.GetBytes() }

// Implements GetXXX for internal consistency
func (b *StatefulBlock[I, O, A]) GetID() ids.ID       { return b.Input.GetID() }
func (b *StatefulBlock[I, O, A]) GetParent() ids.ID   { return b.Input.GetParent() }
func (b *StatefulBlock[I, O, A]) GetHeight() uint64   { return b.Input.GetHeight() }
func (b *StatefulBlock[I, O, A]) GetTimestamp() int64 { return b.Input.GetTimestamp() }
func (b *StatefulBlock[I, O, A]) GetBytes() []byte    { return b.Input.GetBytes() }

// implements "fmt.Stringer"
func (b *StatefulBlock[I, O, A]) String() string {
	return fmt.Sprintf("(%s, verified = %t, accepted = %t)", b.Input, b.verified, b.accepted)
}
