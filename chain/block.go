// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"github.com/ava-labs/hypersdk/workers"
)

var (
	_ snowman.Block      = &StatelessBlock{}
	_ block.StateSummary = &SyncableBlock{}
)

type StatefulBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	Txs []*Transaction `json:"txs"`

	// StateRoot is the root of the post-execution state
	// of [Prnt].
	//
	// This "deferred root" design allows for merklization
	// to be done asynchronously instead of during [Build]
	// or [Verify], which reduces the amount of time we are
	// blocking the consensus engine from voting on the block,
	// starting the verification of another block, etc.
	StateRoot ids.ID `json:"stateRoot"`

	size int

	// authCounts can be used by batch signature verification
	// to preallocate memory
	authCounts map[uint8]int
}

func (b *StatefulBlock) Size() int {
	return b.size
}

func (b *StatefulBlock) ID() (ids.ID, error) {
	blk, err := b.Marshal()
	if err != nil {
		return ids.ID{}, err
	}
	return utils.ToID(blk), nil
}

func NewGenesisBlock(root ids.ID) *StatefulBlock {
	return &StatefulBlock{
		// We set the genesis block timestamp to be after the ProposerVM fork activation.
		//
		// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
		// where the child timestamp is rounded down to the nearest second (which may be before
		// the timestamp of its parent, which is denoted in milliseconds).
		//
		// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
		// .../vms/proposervm/pre_fork_block.go#L201
		Tmstmp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),

		// StateRoot should include all allocates made when loading the genesis file
		StateRoot: root,
	}
}

// Stateless is defined separately from "Block"
// in case external packages needs use the stateful block
// without mocking VM or parent block
type StatelessBlock struct {
	*StatefulBlock `json:"block"`

	id     ids.ID
	st     choices.Status
	t      time.Time
	bytes  []byte
	txsSet set.Set[ids.ID]

	results    []*Result
	feeManager *fees.Manager

	vm   VM
	view merkledb.View

	sigJob workers.Job
}

func NewBlock(vm VM, parent snowman.Block, tmstp int64) *StatelessBlock {
	return &StatelessBlock{
		StatefulBlock: &StatefulBlock{
			Prnt:   parent.ID(),
			Tmstmp: tmstp,
			Hght:   parent.Height() + 1,
		},
		vm: vm,
		st: choices.Processing,
	}
}

func ParseBlock(
	ctx context.Context,
	source []byte,
	status choices.Status,
	vm VM,
) (*StatelessBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseBlock")
	defer span.End()

	blk, err := UnmarshalBlock(source, vm)
	if err != nil {
		return nil, err
	}
	// Not guaranteed that a parsed block is verified
	return ParseStatefulBlock(ctx, blk, source, status, vm)
}

// populateTxs is only called on blocks we did not build
func (b *StatelessBlock) populateTxs(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.populateTxs")
	defer span.End()

	// Setup signature verification job
	_, sigVerifySpan := b.vm.Tracer().Start(ctx, "StatelessBlock.verifySignatures") //nolint:spancheck
	job, err := b.vm.AuthVerifiers().NewJob(len(b.Txs))
	if err != nil {
		return err //nolint:spancheck
	}
	b.sigJob = job
	batchVerifier := NewAuthBatch(b.vm, b.sigJob, b.authCounts)

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
			return ErrDuplicateTx
		}
		b.txsSet.Add(tx.ID())

		// Verify signature async
		if b.vm.GetVerifyAuth() {
			txDigest, err := tx.Digest()
			if err != nil {
				return err
			}
			batchVerifier.Add(txDigest, tx.Auth)
		}
	}
	return nil
}

func ParseStatefulBlock(
	ctx context.Context,
	blk *StatefulBlock,
	source []byte,
	status choices.Status,
	vm VM,
) (*StatelessBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseStatefulBlock")
	defer span.End()

	// Perform basic correctness checks before doing any expensive work
	if blk.Tmstmp > time.Now().Add(FutureBound).UnixMilli() {
		return nil, ErrTimestampTooLate
	}

	if len(source) == 0 {
		nsource, err := blk.Marshal()
		if err != nil {
			return nil, err
		}
		source = nsource
	}
	b := &StatelessBlock{
		StatefulBlock: blk,
		t:             time.UnixMilli(blk.Tmstmp),
		bytes:         source,
		st:            status,
		vm:            vm,
		id:            utils.ToID(source),
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

// [initializeBuilt] is invoked after a block is built
func (b *StatelessBlock) initializeBuilt(
	ctx context.Context,
	view merkledb.View,
	results []*Result,
	feeManager *fees.Manager,
) error {
	_, span := b.vm.Tracer().Start(ctx, "StatelessBlock.initializeBuilt")
	defer span.End()

	blk, err := b.StatefulBlock.Marshal()
	if err != nil {
		return err
	}
	b.bytes = blk
	b.id = utils.ToID(b.bytes)
	b.view = view
	b.t = time.UnixMilli(b.StatefulBlock.Tmstmp)
	b.results = results
	b.feeManager = feeManager
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		b.txsSet.Add(tx.ID())
	}
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) ID() ids.ID { return b.id }

// implements "snowman.Block"
func (b *StatelessBlock) Verify(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.RecordBlockVerify(time.Since(start))
	}()

	stateReady := b.vm.StateReady()
	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessBlock.Verify",
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
			"skipping verification, already processed",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
		)
	default:
		// Get the [VerifyContext] needed to process this block.
		//
		// If the parent block's height is less than or equal to the last accepted height (and
		// the last accepted height is processed), the accepted state will be used as the execution
		// context. Otherwise, the parent block will be used as the execution context.
		vctx, err := b.vm.GetVerifyContext(ctx, b.Hght, b.Prnt)
		if err != nil {
			b.vm.Logger().Warn("unable to get verify context",
				zap.Uint64("height", b.Hght),
				zap.Stringer("blkID", b.ID()),
				zap.Error(err),
			)
			return fmt.Errorf("%w: unable to load verify context", err)
		}

		// Parent block may not be processed when we verify this block, so [innerVerify] may
		// recursively verify ancestry.
		if err := b.innerVerify(ctx, vctx); err != nil {
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

// innerVerify executes the block on top of the provided [VerifyContext].
//
// Invariants:
// Accepted / Rejected blocks should never have Verify called on them.
// Blocks that were verified (and returned nil) with Verify will not have verify called again.
// Blocks that were verified with VerifyWithContext may have verify called multiple times.
//
// When this may be called:
//  1. [Verify|VerifyWithContext]
//  2. If the parent view is missing when verifying (dynamic state sync)
//  3. If the view of a block we are accepting is missing (finishing dynamic
//     state sync)
func (b *StatelessBlock) innerVerify(ctx context.Context, vctx VerifyContext) error {
	var (
		log = b.vm.Logger()
		r   = b.vm.Rules(b.Tmstmp)
	)

	// Perform basic correctness checks before doing any expensive work
	if b.Timestamp().UnixMilli() > time.Now().Add(FutureBound).UnixMilli() {
		return ErrTimestampTooLate
	}

	// Fetch view where we will apply block state transitions
	//
	// This call may result in our ancestry being verified.
	parentView, err := vctx.View(ctx, true)
	if err != nil {
		return fmt.Errorf("%w: unable to load parent view", err)
	}

	// Fetch parent height key and ensure block height is valid
	heightKey := HeightKey(b.vm.StateManager().HeightKey())
	parentHeightRaw, err := parentView.GetValue(ctx, heightKey)
	if err != nil {
		return err
	}
	parentHeight := binary.BigEndian.Uint64(parentHeightRaw)
	if b.Hght != parentHeight+1 {
		return ErrInvalidBlockHeight
	}

	// Fetch parent timestamp and confirm block timestamp is valid
	//
	// Parent may not be available (if we preformed state sync), so we
	// can't rely on being able to fetch it during verification.
	timestampKey := TimestampKey(b.vm.StateManager().TimestampKey())
	parentTimestampRaw, err := parentView.GetValue(ctx, timestampKey)
	if err != nil {
		return err
	}
	parentTimestamp := int64(binary.BigEndian.Uint64(parentTimestampRaw))
	if b.Tmstmp < parentTimestamp+r.GetMinBlockGap() {
		return ErrTimestampTooEarly
	}
	if len(b.Txs) == 0 && b.Tmstmp < parentTimestamp+r.GetMinEmptyBlockGap() {
		return ErrTimestampTooEarly
	}

	// Ensure tx cannot be replayed
	//
	// Before node is considered ready (emap is fully populated), this may return
	// false when other validators think it is true.
	//
	// If a block is already accepted, its transactions have already been added
	// to the VM's seen emap and calling [IsRepeat] will return a non-zero value.
	if b.st != choices.Accepted {
		oldestAllowed := b.Tmstmp - r.GetValidityWindow()
		if oldestAllowed < 0 {
			// Can occur if verifying genesis
			oldestAllowed = 0
		}
		dup, err := vctx.IsRepeat(ctx, oldestAllowed, b.Txs, set.NewBits(), true)
		if err != nil {
			return err
		}
		if dup.Len() > 0 {
			return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
		}
	}

	// Compute next unit prices to use
	feeKey := FeeKey(b.vm.StateManager().FeeKey())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return err
	}
	parentFeeManager := fees.NewManager(feeRaw)
	feeManager, err := parentFeeManager.ComputeNext(parentTimestamp, b.Tmstmp, r)
	if err != nil {
		return err
	}

	// Process transactions
	results, ts, err := b.Execute(ctx, b.vm.Tracer(), parentView, feeManager, r)
	if err != nil {
		log.Error("failed to execute block", zap.Error(err))
		return err
	}
	b.results = results
	b.feeManager = feeManager

	// Update chain metadata
	heightKeyStr := string(heightKey)
	timestampKeyStr := string(timestampKey)
	feeKeyStr := string(feeKey)

	keys := make(state.Keys)
	keys.Add(heightKeyStr, state.Write)
	keys.Add(timestampKeyStr, state.Write)
	keys.Add(feeKeyStr, state.Write)
	tsv := ts.NewView(keys, map[string][]byte{
		heightKeyStr:    parentHeightRaw,
		timestampKeyStr: parentTimestampRaw,
		feeKeyStr:       parentFeeManager.Bytes(),
	})
	if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return err
	}
	if err := tsv.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(b.Tmstmp))); err != nil {
		return err
	}
	if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
		return err
	}
	tsv.Commit()

	// Compare state root
	//
	// Because fee bytes are not recorded in state, it is sufficient to check the state root
	// to verify all fee calcuations were correct.
	_, rspan := b.vm.Tracer().Start(ctx, "StatelessBlock.Verify.WaitRoot")
	start := time.Now()
	computedRoot, err := parentView.GetMerkleRoot(ctx)
	rspan.End()
	if err != nil {
		return err
	}
	b.vm.RecordWaitRoot(time.Since(start))
	if b.StateRoot != computedRoot {
		return fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			b.StateRoot,
		)
	}

	// Ensure signatures are verified
	_, sspan := b.vm.Tracer().Start(ctx, "StatelessBlock.Verify.WaitSignatures")
	start = time.Now()
	err = b.sigJob.Wait()
	sspan.End()
	if err != nil {
		return err
	}
	b.vm.RecordWaitSignatures(time.Since(start))

	// Get view from [tstate] after processing all state transitions
	b.vm.RecordStateChanges(ts.PendingChanges())
	b.vm.RecordStateOperations(ts.OpIndex())
	view, err := ts.ExportMerkleDBView(ctx, b.vm.Tracer(), parentView)
	if err != nil {
		return err
	}
	b.view = view

	// Kickoff root generation
	go func() {
		start := time.Now()
		root, err := view.GetMerkleRoot(ctx)
		if err != nil {
			log.Error("merkle root generation failed", zap.Error(err))
			return
		}
		log.Info("merkle root generated",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
			zap.Stringer("root", root),
		)
		b.vm.RecordRootCalculated(time.Since(start))
	}()
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) Accept(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.RecordBlockAccept(time.Since(start))
	}()

	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.Accept")
	defer span.End()

	// Consider verifying the a block if it is not processed and we are no longer
	// syncing.
	if !b.Processed() {
		// The state of this block was not calculated during the call to
		// [StatelessBlock.Verify]. This is because the VM was state syncing
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
		vctx, err := b.vm.GetVerifyContext(ctx, b.Hght, b.Prnt)
		if err != nil {
			return fmt.Errorf("%w: unable to get verify context", err)
		}
		if err := b.innerVerify(ctx, vctx); err != nil {
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

func (b *StatelessBlock) MarkAccepted(ctx context.Context) {
	// Accept block and free unnecessary memory
	b.st = choices.Accepted
	b.txsSet = nil // only used for replay protection when processing

	// [Accepted] will persist the block to disk and set in-memory variables
	// needed to ensure we don't resync all blocks when state sync finishes.
	//
	// Note: We will not call [b.vm.Verified] before accepting during state sync
	b.vm.Accepted(ctx, b)
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) Reject(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.Reject")
	defer span.End()

	b.st = choices.Rejected
	b.vm.Rejected(ctx, b)
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) Status() choices.Status { return b.st }

// implements "snowman.Block"
func (b *StatelessBlock) Parent() ids.ID { return b.StatefulBlock.Prnt }

// implements "snowman.Block"
func (b *StatelessBlock) Bytes() []byte { return b.bytes }

// implements "snowman.Block"
func (b *StatelessBlock) Height() uint64 { return b.StatefulBlock.Hght }

// implements "snowman.Block"
func (b *StatelessBlock) Timestamp() time.Time { return b.t }

// Used to determine if should notify listeners and/or pass to controller
func (b *StatelessBlock) Processed() bool {
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
func (b *StatelessBlock) View(ctx context.Context, verify bool) (state.View, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.View",
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
		if b.st == choices.Accepted {
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
	if b.st == choices.Accepted {
		acceptedState, err := b.vm.State()
		if err != nil {
			return nil, err
		}
		acceptedHeightRaw, err := acceptedState.Get(HeightKey(b.vm.StateManager().HeightKey()))
		if err != nil {
			return nil, err
		}
		acceptedHeight := binary.BigEndian.Uint64(acceptedHeightRaw)
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
		return nil, ErrBlockNotProcessed
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
		zap.Bool("accepted", b.st == choices.Accepted),
	)
	vctx, err := b.vm.GetVerifyContext(ctx, b.Hght, b.Prnt)
	if err != nil {
		b.vm.Logger().Error("unable to get verify context", zap.Error(err))
		return nil, err
	}
	if err := b.innerVerify(ctx, vctx); err != nil {
		b.vm.Logger().Error("unable to verify block", zap.Error(err))
		return nil, err
	}
	if b.st != choices.Accepted {
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
func (b *StatelessBlock) IsRepeat(
	ctx context.Context,
	oldestAllowed int64,
	txs []*Transaction,
	marker set.Bits,
	stop bool,
) (set.Bits, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.IsRepeat")
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
	if b.st == choices.Accepted || b.Hght == 0 /* genesis */ {
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
	prnt, err := b.vm.GetStatelessBlock(ctx, b.Prnt)
	if err != nil {
		return marker, err
	}
	return prnt.IsRepeat(ctx, oldestAllowed, txs, marker, stop)
}

func (b *StatelessBlock) GetTxs() []*Transaction {
	return b.Txs
}

func (b *StatelessBlock) GetTimestamp() int64 {
	return b.Tmstmp
}

func (b *StatelessBlock) Results() []*Result {
	return b.results
}

func (b *StatelessBlock) FeeManager() *fees.Manager {
	return b.feeManager
}

func (b *StatefulBlock) Marshal() ([]byte, error) {
	size := consts.IDLen + consts.Uint64Len + consts.Uint64Len +
		consts.Uint64Len + window.WindowSliceSize +
		consts.IntLen + codec.CummSize(b.Txs) +
		consts.IDLen + consts.Uint64Len + consts.Uint64Len

	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackInt(len(b.Txs))
	b.authCounts = map[uint8]int{}
	for _, tx := range b.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
		b.authCounts[tx.Auth.GetTypeID()]++
	}

	p.PackID(b.StateRoot)
	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	b.size = len(bytes)
	return bytes, nil
}

func UnmarshalBlock(raw []byte, parser Parser) (*StatefulBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b StatefulBlock
	)
	b.size = len(raw)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	// Parse transactions
	txCount := p.UnpackInt(false) // can produce empty blocks
	actionRegistry, authRegistry := parser.Registry()
	b.Txs = []*Transaction{} // don't preallocate all to avoid DoS
	b.authCounts = map[uint8]int{}
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		b.Txs = append(b.Txs, tx)
		b.authCounts[tx.Auth.GetTypeID()]++
	}

	p.UnpackID(false, &b.StateRoot)

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &b, p.Err()
}

type SyncableBlock struct {
	*StatelessBlock
}

func (sb *SyncableBlock) Accept(ctx context.Context) (block.StateSyncMode, error) {
	return sb.vm.AcceptedSyncableBlock(ctx, sb)
}

func NewSyncableBlock(sb *StatelessBlock) *SyncableBlock {
	return &SyncableBlock{sb}
}

func (sb *SyncableBlock) String() string {
	return fmt.Sprintf("%d:%s root=%s", sb.Height(), sb.ID(), sb.StateRoot)
}

// Testing
func (b *StatelessBlock) MarkUnprocessed() {
	b.view = nil
}
