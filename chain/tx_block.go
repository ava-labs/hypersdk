// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"github.com/ava-labs/hypersdk/workers"
)

type TxBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	UnitPrice  uint64        `json:"unitPrice"`
	UnitWindow window.Window `json:"unitWindow"`

	// PChainHeight is needed to verify warp messages
	ContainsWarp bool   `json:"containsWarp"`
	PChainHeight uint64 `json:"pchainHeight"`

	Txs           []*Transaction `json:"txs"`
	WarpResults   set.Bits64     `json:"warpResults"`
	UnitsConsumed uint64         `json:"unitsConsumed"`
	Last          bool           `json:"last"`

	// TEMP
	Issued int64 `json:"issued"`
}

// Stateless is defined separately from "Block"
// in case external packages needs use the stateful block
// without mocking VM or parent block
type StatelessTxBlock struct {
	*TxBlock `json:"block"`

	id     ids.ID
	t      time.Time
	bytes  []byte
	txsSet set.Set[ids.ID]

	warpMessages map[ids.ID]*warpJob
	vdrState     validators.State

	results []*Result

	vm        VM
	state     merkledb.TrieView
	processor *Processor

	sigJob *workers.Job
}

func NewGenesisTxBlock(minUnitPrice uint64) *TxBlock {
	return &TxBlock{
		UnitPrice:  minUnitPrice,
		UnitWindow: window.Window{},

		Last: true,
	}
}

func NewTxBlock(ectx *TxExecutionContext, vm VM, parent *StatelessTxBlock, tmstmp int64) *StatelessTxBlock {
	return &StatelessTxBlock{
		TxBlock: &TxBlock{
			Tmstmp: tmstmp,
			Prnt:   parent.ID(),
			Hght:   parent.Hght + 1,

			UnitPrice:  ectx.NextUnitPrice,
			UnitWindow: ectx.NextUnitWindow,
		},
		vm: vm,
	}
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

// populateTxs is only called on blocks we did not build
func (b *StatelessTxBlock) populateTxs(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.populateTxs")
	defer span.End()

	// Setup signature verification job
	var sspan oteltrace.Span
	if b.vm.GetVerifySignatures() {
		job, err := b.vm.Workers().NewJob(len(b.Txs))
		if err != nil {
			return err
		}
		b.sigJob = job
		_, sspan = b.vm.Tracer().Start(ctx, "StatelessBlock.verifySignatures")
	}

	// Process transactions
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	b.warpMessages = map[ids.ID]*warpJob{}
	for _, tx := range b.Txs {
		if b.vm.GetVerifySignatures() {
			b.sigJob.Go(tx.AuthAsyncVerify())
		}
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
			if !b.ContainsWarp {
				return ErrWarpResultMismatch
			}
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
		}
	}
	if len(b.warpMessages) > 0 && !b.ContainsWarp {
		return ErrWarpResultMismatch
	}
	if b.vm.GetVerifySignatures() {
		b.sigJob.Done(func() { sspan.End() })
	}
	return nil
}

func ParseTxBlock(
	ctx context.Context,
	blk *TxBlock,
	source []byte,
	vm VM,
) (*StatelessTxBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseTxBlock")
	defer span.End()

	// Perform basic correctness checks before doing any expensive work
	if blk.Hght > 0 {
		if blk.Tmstmp >= time.Now().Add(FutureBound).Unix() {
			return nil, ErrTimestampTooLate
		}
		if len(blk.Txs) == 0 {
			return nil, ErrNoTxs
		}
		r := vm.Rules(blk.Tmstmp)
		if blk.UnitsConsumed > r.GetMaxTxBlockUnits() {
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
	b := &StatelessTxBlock{
		TxBlock: blk,
		t:       time.Unix(blk.Tmstmp, 0),
		bytes:   source,
		vm:      vm,
		id:      utils.ToID(source),
	}

	// If we are parsing an older block, it will not be re-executed and should
	// not be tracked as a parsed block
	lastAccepted := b.vm.LastAcceptedBlock()
	if lastAccepted == nil || b.Hght <= lastAccepted.MaxTxHght() { // nil when parsing genesis
		return b, nil
	}

	// Populate hashes and tx set
	b.vm.RecordTxBlockIssuanceDiff(time.Since(time.UnixMilli(b.Issued)))
	return b, b.populateTxs(ctx)
}

// [initializeBuilt] is invoked after a block is built
func (b *StatelessTxBlock) initializeBuilt(
	ctx context.Context,
	state merkledb.TrieView,
	results []*Result,
) error {
	_, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.initializeBuilt")
	defer span.End()

	blk, err := b.TxBlock.Marshal(b.vm.Registry())
	if err != nil {
		return err
	}
	b.bytes = blk
	b.id = utils.ToID(b.bytes)
	b.t = time.Unix(b.TxBlock.Tmstmp, 0)
	b.state = state
	b.results = results
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		b.txsSet.Add(tx.ID())
	}
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessTxBlock) ID() ids.ID { return b.id }

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
func (b *StatelessTxBlock) verifyWarpMessage(ctx context.Context, r Rules, msg *warp.Message) bool {
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
		b.PChainHeight,
		num,
		denom,
	); err != nil {
		b.vm.Logger().
			Warn("unable to verify warp message", zap.Stringer("warpID", warpID), zap.Error(err))
		return false
	}
	return true
}

func (b *StatelessTxBlock) Verify(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessTxBlock.Verify",
		oteltrace.WithAttributes(
			attribute.Int("txs", len(b.Txs)),
			attribute.Int64("height", int64(b.Hght)),
			attribute.Bool("containsWarp", b.ContainsWarp),
			attribute.Int64("pchainHeight", int64(b.PChainHeight)),
		),
	)
	defer span.End()

	var (
		log   = b.vm.Logger()
		r     = b.vm.Rules(b.Tmstmp)
		start = time.Now()
	)

	// Perform basic correctness checks before doing any expensive work
	switch {
	case b.Tmstmp >= time.Now().Add(FutureBound).Unix():
		return ErrTimestampTooLate
	case len(b.Txs) == 0:
		return ErrNoTxs
	case b.UnitsConsumed > r.GetMaxTxBlockUnits():
		return ErrBlockTooBig
	}

	// Verify parent is verified and available
	var prntHght uint64
	if b.Hght > 0 {
		prntHght = b.Hght - 1
	}
	parent, err := b.vm.GetStatelessTxBlock(ctx, b.Prnt, prntHght)
	if err != nil {
		log.Warn("could not get parent", zap.Stringer("id", b.Prnt))
		return err
	}
	if b.Tmstmp < parent.Tmstmp {
		return ErrTimestampTooEarly
	}
	ectx, err := GenerateTxExecutionContext(ctx, b.vm.ChainID(), b.Tmstmp, parent, b.vm.Tracer(), r)
	if err != nil {
		return err
	}
	switch {
	case b.UnitPrice != ectx.NextUnitPrice:
		return ErrInvalidUnitPrice
	case b.UnitWindow != ectx.NextUnitWindow:
		b.vm.Logger().Warn("unit window mismatch", zap.Uint64("height", b.Hght), zap.Bool("parent", parent != nil), zap.Binary("found", b.UnitWindow[:]), zap.Binary("expected", ectx.NextUnitWindow[:]))
		return ErrInvalidUnitWindow
	}
	log.Info(
		"verify context",
		zap.Uint64("height", b.Hght),
		zap.Uint64("unit price", b.UnitPrice),
	)

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
		return err
	}
	if dup {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}

	// Start validating warp messages, if they exist
	var invalidWarpResult bool
	if b.ContainsWarp {
		_, sspan := b.vm.Tracer().Start(ctx, "StatelessTxBlock.verifyWarpMessages")
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

	// Optimisticaly fetch state
	if parent.Last {
		state, err := parent.ChildState(ctx, 30_000)
		if err != nil {
			return err
		}
		b.processor = NewProcessor(b.vm.Tracer(), state, tstate.New(30_000))
	} else {
		b.processor = parent.processor
	}
	b.processor.Prefetch(ctx, b)

	// Process new transactions
	unitsConsumed, results, stateChanges, stateOps, err := b.processor.Execute(ctx, ectx, r)
	if err != nil {
		log.Error("failed to execute block", zap.Error(err))
		return err
	}
	b.vm.RecordStateChanges(stateChanges)
	b.vm.RecordStateOperations(stateOps)
	if b.UnitsConsumed != unitsConsumed {
		return fmt.Errorf(
			"%w: required=%d found=%d",
			ErrInvalidUnitsConsumed,
			unitsConsumed,
			b.UnitsConsumed,
		)
	}

	// Ensure warp results are correct
	if invalidWarpResult {
		return ErrWarpResultMismatch
	}
	numWarp := len(b.warpMessages)
	if numWarp > MaxWarpMessages {
		return ErrTooManyWarpMessages
	}
	var warpResultsLimit set.Bits64
	warpResultsLimit.Add(uint(numWarp))
	if b.WarpResults >= warpResultsLimit {
		// If the value of [WarpResults] is greater than the value of uint64 with
		// a 1-bit shifted [numWarp] times, then there are unused bits set to
		// 1 (which should is not allowed).
		return ErrWarpResultMismatch
	}

	// We purposely avoid root calc (which would make this spikey)
	b.vm.RecordTxBlockVerify(time.Since(start))

	// Compute state root if last
	//
	// TODO: better protect against malicious usage of this (may need to be
	// invoked by caller block)
	if b.Last {
		// Commit to base
		if err := b.processor.Commit(ctx); err != nil {
			return err
		}
		// Store height in state to prevent duplicate roots
		base := b.processor.db
		if err := base.Insert(ctx, b.vm.StateManager().HeightKey(), binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
			return err
		}
		start := time.Now()
		if _, err := base.GetMerkleRoot(ctx); err != nil {
			return err
		}
		b.vm.RecordRootCalculated(time.Since(start))
		b.state = base
	}

	// We wait for signatures in root block.
	b.results = results
	return nil
}

func (b *StatelessTxBlock) Accept(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.Accept")
	defer span.End()

	b.txsSet = nil // only used for replay protection when processing
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessTxBlock) Reject(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.Reject")
	defer span.End()
	return nil
}

// implements "snowman.Block"
func (b *StatelessTxBlock) Parent() ids.ID { return b.TxBlock.Prnt }

// implements "snowman.Block"
func (b *StatelessTxBlock) Bytes() []byte { return b.bytes }

// implements "snowman.Block"
func (b *StatelessTxBlock) Height() uint64 { return b.TxBlock.Hght }

// implements "snowman.Block"
func (b *StatelessTxBlock) Timestamp() time.Time { return b.t }

func (b *StatelessTxBlock) IsRepeat(
	ctx context.Context,
	oldestAllowed int64,
	txs []*Transaction,
) (bool, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.IsRepeat")
	defer span.End()

	// Early exit if we are already back at least [ValidityWindow]
	if b.Tmstmp < oldestAllowed {
		return false, nil
	}

	// If we are at an accepted block or genesis, we can use the emap on the VM
	// instead of checking each block
	if b.Hght <= b.vm.LastAcceptedBlock().MaxTxHght() {
		return b.vm.IsRepeat(ctx, txs), nil
	}

	// Check if block contains any overlapping txs
	for _, tx := range txs {
		if b.txsSet.Contains(tx.ID()) {
			return true, nil
		}
	}
	var prntHght uint64
	if b.Hght > 0 {
		// TODO: consider making a helper
		prntHght = b.Hght - 1
	}
	prnt, err := b.vm.GetStatelessTxBlock(ctx, b.Prnt, prntHght)
	if err != nil {
		return false, err
	}
	return prnt.IsRepeat(ctx, oldestAllowed, txs)
}

func (b *StatelessTxBlock) GetTxs() []*Transaction {
	return b.Txs
}

func (b *StatelessTxBlock) GetTimestamp() int64 {
	return b.Tmstmp
}

func (b *StatelessTxBlock) GetUnitPrice() uint64 {
	return b.UnitPrice
}

func (b *StatelessTxBlock) Results() []*Result {
	return b.results
}

// We assume this will only be called once we are done syncing, so it is safe
// to assume we will eventually get to a block with state.
func (b *StatelessTxBlock) ChildState(
	ctx context.Context,
	estimatedChanges int,
) (merkledb.TrieView, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.childState")
	defer span.End()

	// Return committed state if block is accepted or this is genesis.
	if b.Hght <= b.vm.LastAcceptedBlock().MaxTxHght() {
		state, err := b.vm.State()
		if err != nil {
			return nil, err
		}
		return state.NewPreallocatedView(estimatedChanges)
	}

	// Process block if not yet processed and not yet accepted.
	//
	// We don't need to handle the case where the tx block is loaded from disk
	// because that will hit the first if check here.
	if b.state == nil {
		return nil, errors.New("not implemented")
	}
	return b.state.NewPreallocatedView(estimatedChanges)
}

func (b *TxBlock) Marshal(
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) ([]byte, error) {
	size := consts.IDLen + consts.Uint64Len + consts.Uint64Len + consts.Uint64Len +
		window.WindowSliceSize + codec.BoolLen + consts.Uint64Len + consts.IntLen
	for _, tx := range b.Txs {
		size += tx.Size()
	}
	size += consts.Uint64Len + consts.Uint64Len + codec.BoolLen + consts.Uint64Len
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackUint64(b.UnitPrice)
	p.PackWindow(b.UnitWindow)

	p.PackBool(b.ContainsWarp)
	p.PackUint64(b.PChainHeight)

	p.PackInt(len(b.Txs))
	for _, tx := range b.Txs {
		if err := tx.Marshal(p, actionRegistry, authRegistry); err != nil {
			return nil, err
		}
	}
	p.PackUint64(uint64(b.WarpResults))
	p.PackUint64(b.UnitsConsumed)
	p.PackBool(b.Last)
	p.PackInt64(b.Issued)

	return p.Bytes(), p.Err()
}

func UnmarshalTxBlock(raw []byte, parser Parser) (*TxBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b TxBlock
	)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	b.UnitPrice = p.UnpackUint64(false)
	p.UnpackWindow(&b.UnitWindow)

	b.ContainsWarp = p.UnpackBool()
	b.PChainHeight = p.UnpackUint64(false)
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
	b.WarpResults = set.Bits64(p.UnpackUint64(false))
	b.UnitsConsumed = p.UnpackUint64(false)
	b.Last = p.UnpackBool()
	b.Issued = p.UnpackInt64(false)

	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
	}
	return &b, p.Err()
}
