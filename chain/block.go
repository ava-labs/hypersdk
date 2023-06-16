// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/AnomalyFi/hypersdk/window"
	"github.com/AnomalyFi/hypersdk/workers"
)

var (
	_ snowman.Block           = &StatelessBlock{}
	_ block.WithVerifyContext = &StatelessBlock{}
	_ block.StateSummary      = &SyncableBlock{}
)

type StatefulBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	UnitPrice  uint64        `json:"unitPrice"`
	UnitWindow window.Window `json:"unitWindow"`

	BlockCost   uint64        `json:"blockCost"`
	BlockWindow window.Window `json:"blockWindow"`

	Txs []*Transaction `json:"txs"`

	StateRoot     ids.ID     `json:"stateRoot"`
	UnitsConsumed uint64     `json:"unitsConsumed"`
	SurplusFee    uint64     `json:"surplusFee"`
	WarpResults   set.Bits64 `json:"warpResults"`
}

// warpJob is used to signal to a listner that a *warp.Message has been
// verified.
type warpJob struct {
	msg          *warp.Message
	signers      int
	verifiedChan chan bool
	verified     bool
	warpNum      int
}

func NewGenesisBlock(root ids.ID, minUnit uint64, minBlock uint64) *StatefulBlock {
	return &StatefulBlock{
		UnitPrice:  minUnit,
		UnitWindow: window.Window{},

		BlockCost:   minBlock,
		BlockWindow: window.Window{},

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

	warpMessages map[ids.ID]*warpJob
	containsWarp bool // this allows us to avoid allocating a map when we build
	bctx         *block.Context
	vdrState     validators.State

	results []*Result

	vm    VM
	state merkledb.TrieView

	sigJob *workers.Job
}

func NewBlock(ectx *ExecutionContext, vm VM, parent snowman.Block, tmstp int64) *StatelessBlock {
	return &StatelessBlock{
		StatefulBlock: &StatefulBlock{
			Prnt:   parent.ID(),
			Tmstmp: tmstp,
			Hght:   parent.Height() + 1,

			UnitPrice:  ectx.NextUnitPrice,
			UnitWindow: ectx.NextUnitWindow,

			BlockCost:   ectx.NextBlockCost,
			BlockWindow: ectx.NextBlockWindow,
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
	job, err := b.vm.Workers().NewJob(len(b.Txs))
	if err != nil {
		return err
	}
	b.sigJob = job

	// Process transactions
	_, sspan := b.vm.Tracer().Start(ctx, "StatelessBlock.verifySignatures")
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	b.warpMessages = map[ids.ID]*warpJob{}
	for _, tx := range b.Txs {
		b.sigJob.Go(tx.AuthAsyncVerify())
		if b.txsSet.Contains(tx.ID()) {
			return ErrDuplicateTx
		}
		b.txsSet.Add(tx.ID())

		// Check if we need the block context to verify the block (which contains
		// an Avalanche Warp Message)
		//
		// Instead of erroring out if a warp message is invalid, we mark the
		// verification as skipped and include it in the verification result so
		// that a fee can still be deducted.
		if tx.WarpMessage != nil {
			if len(b.warpMessages) == MaxWarpMessages {
				return ErrTooManyWarpMessages
			}
			signers, err := tx.WarpMessage.Signature.NumSigners()
			if err != nil {
				return err
			}
			b.warpMessages[tx.ID()] = &warpJob{
				msg:          tx.WarpMessage,
				signers:      signers,
				verifiedChan: make(chan bool, 1),
				warpNum:      len(b.warpMessages),
			}
			b.containsWarp = true
		}
	}
	b.sigJob.Done(func() { sspan.End() })
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
	if blk.Hght > 0 { // skip genesis
		if blk.Tmstmp >= time.Now().Add(FutureBound).Unix() {
			return nil, ErrTimestampTooLate
		}
		if len(blk.Txs) == 0 {
			return nil, ErrNoTxs
		}
		r := vm.Rules(blk.Tmstmp)
		if len(blk.Txs) > r.GetMaxBlockTxs() {
			return nil, ErrBlockTooBig
		}
	}

	if len(source) == 0 {
		actionRegistry, authRegistry := vm.Registry()
		nsource, err := blk.Marshal(actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		source = nsource
	}
	b := &StatelessBlock{
		StatefulBlock: blk,
		t:             time.Unix(blk.Tmstmp, 0),
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
	state merkledb.TrieView,
	results []*Result,
) error {
	_, span := b.vm.Tracer().Start(ctx, "StatelessBlock.initializeBuilt")
	defer span.End()

	blk, err := b.StatefulBlock.Marshal(b.vm.Registry())
	if err != nil {
		return err
	}
	b.bytes = blk
	b.id = utils.ToID(b.bytes)
	b.state = state
	b.t = time.Unix(b.StatefulBlock.Tmstmp, 0)
	b.results = results
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		b.txsSet.Add(tx.ID())
		if tx.WarpMessage != nil {
			b.containsWarp = true
		}
	}
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) ID() ids.ID { return b.id }

// implements "block.WithVerifyContext"
func (b *StatelessBlock) ShouldVerifyWithContext(context.Context) (bool, error) {
	return b.containsWarp, nil
}

// implements "block.WithVerifyContext"
func (b *StatelessBlock) VerifyWithContext(ctx context.Context, bctx *block.Context) error {
	stateReady := b.vm.StateReady()
	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessBlock.VerifyWithContext",
		oteltrace.WithAttributes(
			attribute.Int("txs", len(b.Txs)),
			attribute.Int64("height", int64(b.Hght)),
			attribute.Bool("stateReady", stateReady),
			attribute.Int64("pchainHeight", int64(bctx.PChainHeight)),
			attribute.Bool("built", b.Processed()),
		),
	)
	defer span.End()

	// Persist the context in case we need it during Accept
	b.bctx = bctx

	// Proceed with normal verification
	return b.verify(ctx, stateReady)
}

// implements "snowman.Block"
func (b *StatelessBlock) Verify(ctx context.Context) error {
	stateReady := b.vm.StateReady()
	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessBlock.Verify",
		oteltrace.WithAttributes(
			attribute.Int("txs", len(b.Txs)),
			attribute.Int64("height", int64(b.Hght)),
			attribute.Bool("stateReady", stateReady),
			attribute.Bool("built", b.Processed()),
		),
	)
	defer span.End()

	return b.verify(ctx, stateReady)
}

func (b *StatelessBlock) verify(ctx context.Context, stateReady bool) error {
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
		// Parent may not be processed when we verify this block so [verify] may
		// recursively compute missing state.
		state, err := b.innerVerify(ctx)
		if err != nil {
			return err
		}
		b.state = state
	}
	

	// At any point after this, we may attempt to verify the block. We should be
	// sure we are prepared to do so.
	//
	// NOTE: mempool is modified by VM handler
	b.vm.Verified(ctx, b)
	return nil
}

func preVerifyWarpMessage(msg *warp.Message, chainID ids.ID, r Rules) (uint64, uint64, error) {
	if msg.DestinationChainID != chainID && msg.DestinationChainID != ids.Empty {
		return 0, 0, ErrInvalidChainID
	}
	if msg.SourceChainID == chainID {
		return 0, 0, ErrInvalidChainID
	}
	if msg.SourceChainID == msg.DestinationChainID {
		return 0, 0, ErrInvalidChainID
	}
	allowed, num, denom := r.GetWarpConfig(msg.SourceChainID)
	if !allowed {
		return 0, 0, ErrDisabledChainID
	}
	return num, denom, nil
}

// verifyWarpMessage will attempt to verify a given warp message provided by an
// Action.
func (b *StatelessBlock) verifyWarpMessage(ctx context.Context, r Rules, msg *warp.Message) bool {
	warpID := utils.ToID(msg.Payload)
	num, denom, err := preVerifyWarpMessage(msg, b.vm.ChainID(), r)
	if err != nil {
		b.vm.Logger().
			Warn("unable to verify warp message", zap.Stringer("warpID", warpID), zap.Error(err))
		return false
	}
	if err := msg.Signature.Verify(
		ctx,
		&msg.UnsignedMessage,
		b.vdrState,
		b.bctx.PChainHeight,
		num,
		denom,
	); err != nil {
		b.vm.Logger().
			Warn("unable to verify warp message", zap.Stringer("warpID", warpID), zap.Error(err))
		return false
	}
	return true
}

// Must handle re-reverification...
//
// Invariants:
// Accepted / Rejected blocks should never have Verify called on them.
// Blocks that were verified (and returned nil) with Verify will not have verify called again.
// Blocks that were verified with VerifyWithContext may have verify called multiple times.
//
// When this may be called:
//  1. [Verify|VerifyWithContext]
//  2. If the parent state is missing when verifying (dynamic state sync)
//  3. If the state of a block we are accepting is missing (finishing dynamic
//     state sync)
func (b *StatelessBlock) innerVerify(ctx context.Context) (merkledb.TrieView, error) {
	var (
		log = b.vm.Logger()
		r   = b.vm.Rules(b.Tmstmp)
	)

	// Perform basic correctness checks before doing any expensive work
	switch {
	case b.Timestamp().Unix() >= time.Now().Add(FutureBound).Unix():
		return nil, ErrTimestampTooLate
	case len(b.Txs) == 0:
		return nil, ErrNoTxs
	case len(b.Txs) > r.GetMaxBlockTxs():
		return nil, ErrBlockTooBig
	}

	// Verify parent is verified and available
	parent, err := b.vm.GetStatelessBlock(ctx, b.Prnt)
	if err != nil {
		log.Debug("could not get parent", zap.Stringer("id", b.Prnt))
		return nil, err
	}
	if b.Timestamp().Unix() < parent.Timestamp().Unix() {
		return nil, ErrTimestampTooEarly
	}

	// Ensure tx cannot be replayed
	//
	// Before node is considered ready (emap is fully populated), this may return
	// false when other validators think it is true.
	oldestAllowed := b.Tmstmp - r.GetValidityWindow()
	if oldestAllowed < 0 {
		// Can occur if verifying genesis
		oldestAllowed = 0
	}
	dup, err := parent.IsRepeat(ctx, oldestAllowed, b.Txs)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}

	ectx, err := GenerateExecutionContext(ctx, b.vm.ChainID(), b.Tmstmp, parent, b.vm.Tracer(), r)
	if err != nil {
		return nil, err
	}
	switch {
	case b.UnitPrice != ectx.NextUnitPrice:
		return nil, ErrInvalidUnitPrice
	case b.UnitWindow != ectx.NextUnitWindow:
		return nil, ErrInvalidUnitWindow
	case b.BlockCost != ectx.NextBlockCost:
		return nil, ErrInvalidBlockCost
	case b.BlockWindow != ectx.NextBlockWindow:
		return nil, ErrInvalidBlockWindow
	}
	log.Info(
		"verify context",
		zap.Uint64("height", b.Hght),
		zap.Uint64("unit price", b.UnitPrice),
		zap.Uint64("block cost", b.BlockCost),
	)

	// Start validating warp messages, if they exist
	var invalidWarpResult bool
	if b.containsWarp {
		if b.bctx == nil {
			log.Error(
				"missing verify block context",
				zap.Uint64("height", b.Hght),
				zap.Stringer("id", b.ID()),
			)
			return nil, ErrMissingBlockContext
		}
		_, sspan := b.vm.Tracer().Start(ctx, "StatelessBlock.verifyWarpMessages")
		b.vdrState = b.vm.ValidatorState()
		go func() {
			defer sspan.End()
			// We don't use [b.vm.Workers] here because we need the warp verification
			// results during normal execution. If we added a job to the workers queue,
			// it would get executed after all signatures. Additionally, BLS
			// Multi-Signature verification is already parallelized so we should just
			// do one at a time to avoid overwhelming the CPU.
			for txID, msg := range b.warpMessages {
				if ctx.Err() != nil {
					return
				}
				blockVerified := b.WarpResults.Contains(uint(msg.warpNum))
				if b.vm.IsBootstrapped() && !invalidWarpResult {
					start := time.Now()
					verified := b.verifyWarpMessage(ctx, r, msg.msg)
					msg.verifiedChan <- verified
					msg.verified = verified
					log.Info(
						"processed warp message",
						zap.Stringer("txID", txID),
						zap.Bool("verified", verified),
						zap.Int("signers", msg.signers),
						zap.Duration("t", time.Since(start)),
					)
					if blockVerified != verified {
						invalidWarpResult = true
					}
				} else {
					// When we are bootstrapping, we just use the result in the block.
					//
					// We also use the result in the block when we have found
					// a verification mismatch (our verify result is different than the
					// block) to avoid doing extra work.
					msg.verifiedChan <- blockVerified
					msg.verified = blockVerified
				}
			}
		}()
	}

	// Fetch parent state
	//
	// This function may verify the parent if it is not yet verified.
	state, err := parent.childState(ctx, len(b.Txs)*2)
	if err != nil {
		return nil, err
	}

	// Optimisticaly fetch state
	processor := NewProcessor(b.vm.Tracer(), b)
	processor.Prefetch(ctx, state)

	// Process new transactions
	unitsConsumed, surplusFee, results, stateChanges, stateOps, err := processor.Execute(ctx, ectx, r)
	if err != nil {
		log.Error("failed to execute block", zap.Error(err))
		return nil, err
	}
	b.vm.RecordStateChanges(stateChanges)
	b.vm.RecordStateOperations(stateOps)
	b.results = results
	if b.UnitsConsumed != unitsConsumed {
		return nil, fmt.Errorf(
			"%w: required=%d found=%d",
			ErrInvalidUnitsConsumed,
			unitsConsumed,
			b.UnitsConsumed,
		)
	}

	// Ensure enough fee is paid to compensate for block production speed
	if b.SurplusFee != surplusFee {
		return nil, fmt.Errorf(
			"%w: required=%d found=%d",
			ErrInvalidSurplus,
			b.SurplusFee,
			surplusFee,
		)
	}
	requiredSurplus := b.UnitPrice * b.BlockCost
	if surplusFee < requiredSurplus {
		return nil, fmt.Errorf(
			"%w: required=%d found=%d",
			ErrInsufficientSurplus,
			requiredSurplus,
			surplusFee,
		)
	}

	// Ensure warp results are correct
	if invalidWarpResult {
		return nil, ErrWarpResultMismatch
	}
	numWarp := len(b.warpMessages)
	if numWarp > MaxWarpMessages {
		return nil, ErrTooManyWarpMessages
	}
	var warpResultsLimit set.Bits64
	warpResultsLimit.Add(uint(numWarp))
	if b.WarpResults >= warpResultsLimit {
		// If the value of [WarpResults] is greater than the value of uint64 with
		// a 1-bit shifted [numWarp] times, then there are unused bits set to
		// 1 (which should is not allowed).
		return nil, ErrWarpResultMismatch
	}

	// Store height in state to prevent duplicate roots
	if err := state.Insert(ctx, b.vm.StateManager().HeightKey(), binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, err
	}

	// Compute state root
	start := time.Now()
	computedRoot, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	b.vm.RecordRootCalculated(time.Since(start))
	if b.StateRoot != computedRoot {
		return nil, fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			b.StateRoot,
		)
	}

	// Ensure signatures are verified
	_, sspan := b.vm.Tracer().Start(ctx, "StatelessBlock.Verify.WaitSignatures")
	defer sspan.End()
	start = time.Now()
	if err := b.sigJob.Wait(); err != nil {
		return nil, err
	}
	b.vm.RecordWaitSignatures(time.Since(start))
	return state, nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) Accept(ctx context.Context) error {
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
			b.vm.Logger().
				Info("updated state sync target", zap.Stringer("id", b.ID()), zap.Stringer("root", b.StateRoot))
			return nil // the sync is still ongoing
		}
		b.vm.Logger().
			Info("verifying unprocessed block in accept", zap.Stringer("id", b.ID()), zap.Stringer("root", b.StateRoot))
		// This check handles the case where blocks were not
		// verified during state sync (stopped syncing with a processing block).
		//
		// If state sync completes before accept is called
		// then we need to rebuild it here.
		state, err := b.innerVerify(ctx)
		if err != nil {
			return err
		}
		b.state = state
	}

	// Commit state if we don't return before here (would happen if we are still
	// syncing)
	if err := b.state.CommitToDB(ctx); err != nil {
		return err
	}

	for _, tx := range b.Txs {
		b.vm.Logger().Info("Accepted tx action data is:", zap.Stringer("type_of_action", reflect.TypeOf(tx.Action)))
	}

	// Set last accepted block
	return b.SetLastAccepted(ctx)
}

// SetLastAccepted is called during [Accept] and at the start and end of state
// sync.
func (b *StatelessBlock) SetLastAccepted(ctx context.Context) error {
	if err := b.vm.SetLastAccepted(b); err != nil {
		return err
	}
	b.st = choices.Accepted
	b.txsSet = nil // only used for replay protection when processing

	// [Accepted] will set in-memory variables needed to ensure we don't resync
	// all blocks when state sync finishes
	//
	// Note: We will not call [b.vm.Verified] before accepting during state sync
	b.vm.Accepted(ctx, b)
	return nil
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

// State is used to verify txs in the mempool. It should never be written to.
//
func (b *StatelessBlock) State() (Database, error) {
	if b.st == choices.Accepted {
		return b.vm.State()
	}
	if b.Processed() {
		return b.state, nil
	}
	return nil, ErrBlockNotProcessed
}

// Used to determine if should notify listeners and/or pass to controller
func (b *StatelessBlock) Processed() bool {
	return b.state != nil
}

// We assume this will only be called once we are done syncing, so it is safe
// to assume we will eventually get to a block with state.
func (b *StatelessBlock) childState(
	ctx context.Context,
	estimatedChanges int,
) (merkledb.TrieView, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.childState")
	defer span.End()

	// Return committed state if block is accepted or this is genesis.
	if b.st == choices.Accepted || b.Hght == 0 /* genesis */ {
		state, err := b.vm.State()
		if err != nil {
			return nil, err
		}
		return state.NewPreallocatedView(estimatedChanges)
	}

	// Process block if not yet processed and not yet accepted.
	if !b.Processed() {
		b.vm.Logger().
			Info("verifying parent when childState requested", zap.Uint64("height", b.Hght))
		state, err := b.innerVerify(ctx)
		if err != nil {
			return nil, err
		}
		b.state = state
	}
	return b.state.NewPreallocatedView(estimatedChanges)
}

func (b *StatelessBlock) IsRepeat(
	ctx context.Context,
	oldestAllowed int64,
	txs []*Transaction,
) (bool, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.IsRepeat")
	defer span.End()

	// Early exit if we are already back at least [ValidityWindow]
	if b.Tmstmp < oldestAllowed {
		return false, nil
	}

	// If we are at an accepted block or genesis, we can use the emap on the VM
	// instead of checking each block
	if b.st == choices.Accepted || b.Hght == 0 /* genesis */ {
		return b.vm.IsRepeat(ctx, txs), nil
	}

	// Check if block contains any overlapping txs
	for _, tx := range txs {
		if b.txsSet.Contains(tx.ID()) {
			return true, nil
		}
	}
	prnt, err := b.vm.GetStatelessBlock(ctx, b.Prnt)
	if err != nil {
		return false, err
	}
	return prnt.IsRepeat(ctx, oldestAllowed, txs)
}

func (b *StatelessBlock) GetTxs() []*Transaction {
	return b.Txs
}

func (b *StatelessBlock) GetTimestamp() int64 {
	return b.Tmstmp
}

func (b *StatelessBlock) GetUnitPrice() uint64 {
	return b.UnitPrice
}

func (b *StatelessBlock) Results() []*Result {
	return b.results
}

func (b *StatefulBlock) Marshal(
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) ([]byte, error) {
	p := codec.NewWriter(consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackUint64(b.UnitPrice)
	p.PackWindow(b.UnitWindow)

	p.PackUint64(b.BlockCost)
	p.PackWindow(b.BlockWindow)

	p.PackInt(len(b.Txs))
	for _, tx := range b.Txs {
		if err := tx.Marshal(p, actionRegistry, authRegistry); err != nil {
			return nil, err
		}
	}

	p.PackID(b.StateRoot)
	p.PackUint64(b.UnitsConsumed)
	p.PackUint64(b.SurplusFee)
	p.PackUint64(uint64(b.WarpResults))
	return p.Bytes(), p.Err()
}

func UnmarshalBlock(raw []byte, parser Parser) (*StatefulBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b StatefulBlock
	)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	b.UnitPrice = p.UnpackUint64(false)
	p.UnpackWindow(&b.UnitWindow)

	b.BlockCost = p.UnpackUint64(false)
	p.UnpackWindow(&b.BlockWindow)
	if err := p.Err(); err != nil {
		// Check that header was parsed properly before unwrapping transactions
		return nil, err
	}

	// Parse transactions
	txCount := p.UnpackInt(false) // could be 0 in genesis
	actionRegistry, authRegistry := parser.Registry()
	b.Txs = []*Transaction{} // don't preallocate all to avoid DoS
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		b.Txs = append(b.Txs, tx)
	}

	p.UnpackID(false, &b.StateRoot)
	b.UnitsConsumed = p.UnpackUint64(false)
	b.SurplusFee = p.UnpackUint64(false)
	b.WarpResults = set.Bits64(p.UnpackUint64(false))

	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
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
