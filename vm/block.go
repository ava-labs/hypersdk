// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ snowman.Block      = (*StatefulBlock)(nil)
	_ block.StateSummary = (*SyncableBlock)(nil)
)

// StatefulBlock is defined separately from "StatelessBlock"
// in case external packages need to use the stateless block
// without mocking VM or parent block
type StatefulBlock struct {
	*chain.ExecutionBlock `json:"block"`

	id       ids.ID
	accepted bool
	t        time.Time
	bytes    []byte
	txsSet   set.Set[ids.ID]

	executedBlock *chain.ExecutedBlock
	results       []*chain.Result
	feeManager    *internalfees.Manager

	vm       *VM
	executor *chain.Chain
	view     merkledb.View

	sigJob workers.Job
}

// func NewBlock(vm *VM, parent snowman.Block, tmstp int64) *StatefulBlock {
// 	return &StatefulBlock{
// 		StatelessBlock: &chain.StatelessBlock{
// 			Prnt:   parent.ID(),
// 			Tmstmp: tmstp,
// 			Hght:   parent.Height() + 1,
// 		},
// 		vm:       vm,
// 		executor: vm.chain,
// 		accepted: false,
// 	}
// }

func ParseBlock(
	ctx context.Context,
	source []byte,
	accepted bool,
	vm *VM,
) (*StatefulBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseBlock")
	defer span.End()

	blk, err := vm.chain.ParseBlock(ctx, source)
	if err != nil {
		return nil, err
	}
	// Not guaranteed that a parsed block is verified
	return ParseStatefulBlock(ctx, blk, source, accepted, vm)
}

// XXX(incomplete)
// populateTxs is only called on blocks we did not build
func (b *StatefulBlock) populateTxs(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatefulBlock.populateTxs")
	defer span.End()

	// Setup signature verification job
	_, sigVerifySpan := b.vm.Tracer().Start(ctx, "StatefulBlock.verifySignatures") //nolint:spancheck
	job, err := b.vm.AuthVerifiers().NewJob(len(b.Txs))
	if err != nil {
		return err //nolint:spancheck
	}
	b.sigJob = job
	batchVerifier := chain.NewAuthBatch(b.vm, b.sigJob, b.AuthCounts)

	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	// Confirm no transaction duplicates and setup
	// AWM processing
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		// Ensure there are no duplicate transactions
		if b.txsSet.Contains(tx.ID()) {
			return chain.ErrDuplicateTx
		}
		b.txsSet.Add(tx.ID())

		// Verify signature async
		if b.vm.GetVerifyAuth() {
			unsignedTxBytes, err := tx.UnsignedBytes()
			if err != nil {
				return err
			}
			batchVerifier.Add(unsignedTxBytes, tx.Auth)
		}
	}
	return nil
}

func ParseStatefulBlock(
	ctx context.Context,
	blk *chain.ExecutionBlock,
	source []byte,
	accepted bool,
	vm *VM,
) (*StatefulBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseStatefulBlock")
	defer span.End()

	// Perform basic correctness checks before doing any expensive work
	if blk.Tmstmp > time.Now().Add(chain.FutureBound).UnixMilli() {
		return nil, chain.ErrTimestampTooLate
	}

	if len(source) == 0 {
		nsource, err := blk.Marshal()
		if err != nil {
			return nil, err
		}
		source = nsource
	}
	b := &StatefulBlock{
		ExecutionBlock: blk,
		t:              time.UnixMilli(blk.Tmstmp),
		bytes:          source,
		accepted:       accepted,
		vm:             vm,
		executor:       vm.chain,
		id:             utils.ToID(source),
	}

	// If we are parsing an older block, it will not be re-executed and should
	// not be tracked as a parsed block
	lastAccepted := b.vm.LastAcceptedBlock()
	if lastAccepted == nil || b.Hght <= lastAccepted.Hght { // nil when parsing genesis
		return b, nil
	}

	// Populate hashes and tx set
	return b, b.populateTxs(ctx)
}

// XXX(incomplete)
// [initializeBuilt] is invoked after a block is built
func (b *StatefulBlock) initializeBuilt(
	ctx context.Context,
	view merkledb.View,
	results []*chain.Result,
	feeManager *internalfees.Manager,
) error {
	_, span := b.vm.Tracer().Start(ctx, "StatefulBlock.initializeBuilt")
	defer span.End()

	blk, err := b.StatelessBlock.Marshal()
	if err != nil {
		return err
	}
	b.bytes = blk
	b.id = utils.ToID(b.bytes)
	b.view = view
	b.t = time.UnixMilli(b.StatelessBlock.Tmstmp)
	executedBlk, err := chain.NewExecutedBlock(b.StatelessBlock, results, feeManager.UnitPrices())
	if err != nil {
		return err
	}
	b.executedBlock = executedBlk
	b.results = results
	b.feeManager = feeManager
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		b.txsSet.Add(tx.ID())
	}
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock) ID() ids.ID { return b.id }

// implements "snowman.Block"
func (b *StatefulBlock) Verify(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.RecordBlockVerify(time.Since(start))
	}()

	stateReady := b.vm.StateReady()
	ctx, span := b.vm.Tracer().Start(
		ctx, "StatefulBlock.Verify",
		trace.WithAttributes(
			attribute.Int("txs", len(b.Txs)),
			attribute.Int64("height", int64(b.Hght)),
			attribute.Bool("stateReady", stateReady),
			attribute.Bool("built", b.Processed()),
		),
	)
	defer span.End()

	log := b.vm.Logger()
	switch {
	case !stateReady:
		// If the state of the accepted tip has not been fully fetched, it is not safe to
		// verify any block.
		log.Info(
			"skipping verification, state not ready",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
		)
	case b.Processed():
		// If we built the block, the state will already be populated and we don't
		// need to compute it (we assume that we built a correct block and it isn't
		// necessary to re-verify anything).
		log.Info(
			"skipping verification of locally built block",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
		)
	default:
		// Parent block may not be processed when we verify this block, so [innerVerify] may
		// recursively verify ancestry.
		if err := b.innerVerify(ctx); err != nil {
			b.vm.Logger().Warn("verification failed",
				zap.Uint64("height", b.Hght),
				zap.Stringer("blkID", b.ID()),
				zap.Error(err),
			)
			return err
		}
	}

	// At any point after this, we may attempt to verify the block. We should be
	// sure we are prepared to do so.
	//
	// NOTE: mempool is modified by VM handler
	b.vm.Verified(ctx, b)
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
func (b *StatefulBlock) innerVerify(ctx context.Context) error {
	vctx, err := b.GetVerifyContext(ctx, b.Hght, b.Prnt)
	if err != nil {
		return err
	}
	parentView, err := vctx.View(ctx, true)
	if err != nil {
		return err
	}
	executedBlock, view, err := b.executor.Execute(ctx, parentView, b.ExecutionBlock)
	if err != nil {
		return err
	}
	b.executedBlock = executedBlock
	b.results = executedBlock.Results
// XXX(incomplete)
	// b.feeManager = feeManager
	b.view = view
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock) Accept(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.RecordBlockAccept(time.Since(start))
	}()

	ctx, span := b.vm.Tracer().Start(ctx, "StatefulBlock.Accept")
	defer span.End()

	// Consider verifying the a block if it is not processed and we are no longer
	// syncing.
	if !b.Processed() {
		// The state of this block was not calculated during the call to
		// [StatefulBlock.Verify]. This is because the VM was state syncing
		// and did not have the state necessary to verify the block.
		updated, err := b.vm.UpdateSyncTarget(b)
		if err != nil {
			return err
		}
		if updated {
			b.vm.Logger().Info("updated state sync target",
				zap.Stringer("id", b.ID()),
				zap.Stringer("root", b.StateRoot),
			)
			return nil // the sync is still ongoing
		}

		// This code handles the case where this block was not
		// verified during state sync (stopped syncing with a
		// processing block).
		//
		// If state sync completes before accept is called
		// then we need to process it here.
		b.vm.Logger().Info("verifying unprocessed block in accept",
			zap.Stringer("id", b.ID()),
			zap.Stringer("root", b.StateRoot),
		)
		if err := b.innerVerify(ctx); err != nil {
			return fmt.Errorf("%w: unable to verify block", err)
		}
	}

	// Commit view if we don't return before here (would happen if we are still
	// syncing)
	if err := b.view.CommitToDB(ctx); err != nil {
		return fmt.Errorf("%w: unable to commit block", err)
	}

	// Mark block as accepted and update last accepted in storage
	b.MarkAccepted(ctx)
	return nil
}

func (b *StatefulBlock) MarkAccepted(ctx context.Context) {
	// Accept block and free unnecessary memory
	b.accepted = true
	b.txsSet = nil // only used for replay protection when processing

	// [Accepted] will persist the block to disk and set in-memory variables
	// needed to ensure we don't resync all blocks when state sync finishes.
	//
	// Note: We will not call [b.vm.Verified] before accepting during state sync
	b.vm.Accepted(ctx, b)
}

// implements "snowman.Block.choices.Decidable"
func (b *StatefulBlock) Reject(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatefulBlock.Reject")
	defer span.End()

	b.vm.Rejected(ctx, b)
	return nil
}

// implements "snowman.Block"
func (b *StatefulBlock) Parent() ids.ID { return b.StatelessBlock.Prnt }

// implements "snowman.Block"
func (b *StatefulBlock) Bytes() []byte { return b.bytes }

// implements "snowman.Block"
func (b *StatefulBlock) Height() uint64 { return b.StatelessBlock.Hght }

// implements "snowman.Block"
func (b *StatefulBlock) Timestamp() time.Time { return b.t }

// Used to determine if should notify listeners and/or pass to controller
func (b *StatefulBlock) Processed() bool {
	return b.view != nil
}

// View returns the [merkledb.TrieView] of the block (representing the state
// post-execution) or returns the accepted state if the block is accepted or
// is height 0 (genesis).
//
// If [b.view] is nil (not processed), this function will either return an error or will
// run verification (depending on whether the height is in [acceptedState]).
//
// We still need to handle returning the accepted state here because
// the [VM] will call [View] on the preferred tip of the chain (whether or
// not it is accepted).
//
// Invariant: [View] with [verify] == true should not be called concurrently, otherwise,
// it will result in undefined behavior.
func (b *StatefulBlock) View(ctx context.Context, verify bool) (state.View, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatefulBlock.View",
		trace.WithAttributes(
			attribute.Bool("processed", b.Processed()),
			attribute.Bool("verify", verify),
		),
	)
	defer span.End()

	// If this is the genesis block, return the base state.
	if b.Hght == 0 {
		return b.vm.State()
	}

	// If block is processed, we can return either the accepted state
	// or its pending view.
	if b.Processed() {
		if b.accepted {
			// We assume that base state was properly updated if this
			// block was accepted (this is not obvious because
			// the accepted state may be that of the parent of the last
			// accepted block right after state sync finishes).
			return b.vm.State()
		}
		return b.view, nil
	}

	// If the block is not processed but [acceptedState] equals the height
	// of the block, we should return the accepted state.
	//
	// This can happen when we are building a child block immediately after
	// restart (latest block will not be considered [Processed] because there
	// will be no attached view from execution).
	//
	// We cannot use the merkle root to check against the accepted state
	// because the block only contains the root of the parent block's post-execution.
	if b.accepted {
		acceptedState, err := b.vm.State()
		if err != nil {
			return nil, err
		}
		acceptedHeightRaw, err := acceptedState.Get(chain.HeightKey(b.vm.MetadataManager().HeightPrefix()))
		if err != nil {
			return nil, err
		}
		acceptedHeight, err := database.ParseUInt64(acceptedHeightRaw)
		if err != nil {
			return nil, err
		}
		if acceptedHeight == b.Hght {
			b.vm.Logger().Info("accepted block not processed but found post-execution state on-disk",
				zap.Uint64("height", b.Hght),
				zap.Stringer("blkID", b.ID()),
				zap.Bool("verify", verify),
			)
			return acceptedState, nil
		}
		b.vm.Logger().Info("accepted block not processed and does not match state on-disk",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
			zap.Bool("verify", verify),
		)
	} else {
		b.vm.Logger().Info("block not processed",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
			zap.Bool("verify", verify),
		)
	}
	if !verify {
		return nil, chain.ErrBlockNotProcessed
	}

	// If there are no processing blocks when state sync finishes,
	// the first block we attempt to verify will reach this execution
	// path.
	//
	// In this scenario, the last accepted block will not be processed
	// and [acceptedState] will correspond to the post-execution state
	// of the new block's grandparent (our parent). To remedy this,
	// we need to process this block to return a valid view.
	b.vm.Logger().Info("verifying block when view requested",
		zap.Uint64("height", b.Hght),
		zap.Stringer("blkID", b.ID()),
		zap.Bool("accepted", b.accepted),
	)
	if err := b.innerVerify(ctx); err != nil {
		b.vm.Logger().Error("unable to verify block", zap.Error(err))
		return nil, err
	}
	if !b.accepted {
		return b.view, nil
	}

	// If the block is already accepted, we should update
	// the accepted state to ensure future calls to [View]
	// return the correct state (now that the block is considered
	// processed).
	//
	// It is not possible to reach this function if this block
	// is not the child of the block whose post-execution state
	// is currently stored on disk, so it is safe to call [CommitToDB].
	if err := b.view.CommitToDB(ctx); err != nil {
		b.vm.Logger().Error("unable to commit to DB", zap.Error(err))
		return nil, err
	}
	return b.vm.State()
}

// IsRepeat returns a bitset of all transactions that are considered repeats in
// the range that spans back to [oldestAllowed].
//
// If [stop] is set to true, IsRepeat will return as soon as the first repeat
// is found (useful for block verification).
func (b *StatefulBlock) IsRepeat(
	ctx context.Context,
	oldestAllowed int64,
	txs []*chain.Transaction,
	marker set.Bits,
	stop bool,
) (set.Bits, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatefulBlock.IsRepeat")
	defer span.End()

	// Early exit if we are already back at least [ValidityWindow]
	//
	// It is critical to ensure this logic is equivalent to [emap] to avoid
	// non-deterministic verification.
	if b.Tmstmp < oldestAllowed {
		return marker, nil
	}

	// If we are at an accepted block or genesis, we can use the emap on the VM
	// instead of checking each block
	if b.accepted || b.Hght == 0 /* genesis */ {
		return b.vm.IsRepeat(ctx, txs, marker, stop), nil
	}

	// Check if block contains any overlapping txs
	for i, tx := range txs {
		if marker.Contains(i) {
			continue
		}
		if b.txsSet.Contains(tx.ID()) {
			marker.Add(i)
			if stop {
				return marker, nil
			}
		}
	}
	prnt, err := b.vm.GetStatefulBlock(ctx, b.Prnt)
	if err != nil {
		return marker, err
	}
	return prnt.IsRepeat(ctx, oldestAllowed, txs, marker, stop)
}

func (b *StatefulBlock) GetVerifyContext(ctx context.Context, blockHeight uint64, parent ids.ID) (VerifyContext, error) {
	// If [blockHeight] is 0, we throw an error because there is no pre-genesis verification context.
	if blockHeight == 0 {
		return nil, errors.New("cannot get context of genesis block")
	}

	// If the parent block is not yet accepted, we should return the block's processing parent (it may
	// or may not be verified yet).
	lastAcceptedBlock := b.vm.LastAcceptedBlock()
	if blockHeight-1 > lastAcceptedBlock.Hght {
		blk, err := b.vm.GetStatefulBlock(ctx, parent)
		if err != nil {
			return nil, err
		}
		return &PendingVerifyContext{blk}, nil
	}

	// If the last accepted block is not yet processed, we can't use the accepted state for the
	// verification context. This could happen if state sync finishes with no processing blocks (we
	// sync to the post-execution state of the parent of the last accepted block, not the post-execution
	// state of the last accepted block).
	//
	// Invariant: When [View] is called on [vm.lastAccepted], the block will be verified and the accepted
	// state will be updated.
	if !lastAcceptedBlock.Processed() && parent == lastAcceptedBlock.ID() {
		return &PendingVerifyContext{lastAcceptedBlock}, nil
	}

	// If the parent block is accepted and processed, we should
	// just use the accepted state as the verification context.
	return &AcceptedVerifyContext{b.vm}, nil
}

func (b *StatefulBlock) GetTxs() []*chain.Transaction {
	return b.Txs
}

func (b *StatefulBlock) GetTimestamp() int64 {
	return b.Tmstmp
}

func (b *StatefulBlock) Results() []*chain.Result {
	return b.results
}

func (b *StatefulBlock) FeeManager() *internalfees.Manager {
	return b.feeManager
}

type SyncableBlock struct {
	*StatefulBlock
}

func (sb *SyncableBlock) Accept(ctx context.Context) (block.StateSyncMode, error) {
	return sb.vm.AcceptedSyncableBlock(ctx, sb)
}

func NewSyncableBlock(sb *StatefulBlock) *SyncableBlock {
	return &SyncableBlock{sb}
}

func (sb *SyncableBlock) String() string {
	return fmt.Sprintf("%d:%s root=%s", sb.Height(), sb.ID(), sb.StateRoot)
}

// Testing
func (b *StatefulBlock) MarkUnprocessed() {
	b.view = nil
}
