// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
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
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/workers"
)

type TxBlock struct {
	Prnt ids.ID `json:"parent"`
	Hght uint64 `json:"height"`

	Tmstmp    int64  `json:"timestamp"`
	UnitPrice uint64 `json:"unitPrice"`

	// PChainHeight is needed to verify warp messages
	ContainsWarp bool   `json:"containsWarp"`
	PChainHeight uint64 `json:"pchainHeight"`

	Txs           []*Transaction `json:"txs"`
	WarpResults   set.Bits64     `json:"warpResults"`
	UnitsConsumed uint64         `json:"unitsConsumed"`
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

	vm VM

	sigJob *workers.Job
}

func NewTxBlock(vm VM, parent *StatelessTxBlock, tmstmp int64, unitPrice uint64) *StatelessTxBlock {
	blk := &TxBlock{
		Tmstmp:    tmstmp,
		UnitPrice: unitPrice,
	}
	if parent != nil {
		blk.Prnt = parent.ID()
		blk.Hght = parent.Hght + 1
	}
	return &StatelessTxBlock{
		TxBlock: blk,
		vm:      vm,
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
	b.sigJob.Done(func() { sspan.End() })
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
	if blk.Tmstmp >= time.Now().Add(FutureBound).Unix() {
		return nil, ErrTimestampTooLate
	}
	// TODO: ensure all block chunks have same timestamp
	// TODO: check unit price is expected
	if len(blk.Txs) == 0 {
		return nil, ErrNoTxs
	}
	r := vm.Rules(blk.Tmstmp)
	if len(blk.Txs) > r.GetMaxBlockTxs() {
		return nil, ErrBlockTooBig
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
	if lastAccepted == nil || b.Hght <= lastAccepted.MinTxHght { // nil when parsing genesis
		return b, nil
	}

	// Populate hashes and tx set
	return b, b.populateTxs(ctx)
}

// [initializeBuilt] is invoked after a block is built
func (b *StatelessTxBlock) initializeBuilt(
	ctx context.Context,
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
	b.results = results
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		b.txsSet.Add(tx.ID())
		if tx.WarpMessage != nil {
			// TODO: probably don't need to set
			b.ContainsWarp = true
		}
	}
	// TODO: set hasCtx
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

func (b *StatelessTxBlock) Verify(ctx context.Context, base merkledb.TrieView) error {
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
		log = b.vm.Logger()
		r   = b.vm.Rules(b.Tmstmp)
	)

	// Perform basic correctness checks before doing any expensive work
	switch {
	case b.Tmstmp >= time.Now().Add(FutureBound).Unix():
		return ErrTimestampTooLate
	case len(b.Txs) == 0:
		return ErrNoTxs
	case len(b.Txs) > r.GetMaxBlockTxs():
		return ErrBlockTooBig
	}

	// Verify parent is verified and available
	parent, err := b.vm.GetStatelessTxBlock(ctx, b.Prnt)
	if err != nil {
		log.Debug("could not get parent", zap.Stringer("id", b.Prnt))
		return err
	}
	if b.Tmstmp < parent.Tmstmp {
		return ErrTimestampTooEarly
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
	processor := NewProcessor(b.vm.Tracer(), b)
	processor.Prefetch(ctx, base)

	// Process new transactions
	// TODO: add execution context
	unitsConsumed, results, stateChanges, stateOps, err := processor.Execute(ctx, nil, r)
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

	// We wait for signatures in root block.
	b.results = results
	return nil
}

func (b *StatelessTxBlock) Accept(ctx context.Context) error {
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
	lastAccepted := b.vm.LastAcceptedBlock()
	// TODO: check if <= or <
	if b.Hght <= lastAccepted.MinTxHght+uint64(len(lastAccepted.Txs)) {
		return b.vm.IsRepeat(ctx, txs), nil
	}

	// Check if block contains any overlapping txs
	for _, tx := range txs {
		if b.txsSet.Contains(tx.ID()) {
			return true, nil
		}
	}
	prnt, err := b.vm.GetStatelessTxBlock(ctx, b.Prnt)
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

func (b *TxBlock) Marshal(
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) ([]byte, error) {
	p := codec.NewWriter(consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackUint64(b.Hght)

	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.UnitPrice)

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

	return p.Bytes(), p.Err()
}

func UnmarshalTxBlock(raw []byte, parser Parser) (*TxBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b TxBlock
	)

	p.UnpackID(false, &b.Prnt)
	b.Hght = p.UnpackUint64(false)

	b.Tmstmp = p.UnpackInt64(false)
	b.UnitPrice = p.UnpackUint64(false)

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

	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
	}
	return &b, p.Err()
}
