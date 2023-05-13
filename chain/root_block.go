package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	_ snowman.Block           = &StatelessRootBlock{}
	_ block.WithVerifyContext = &StatelessRootBlock{}
	_ block.StateSummary      = &SyncableBlock{}
)

// Chain architecture
//
// Non-Consensus: [TB1] -> [TB2] -> [TB3] -> [TB4] -> [TB5]
// Consensus:                   \-> [RB1]                 \-> [RB2]
type RootBlock struct {
	Prnt   ids.ID `json:"parent"`
	Tmstmp int64  `json:"timestamp"`
	Hght   uint64 `json:"height"`

	UnitPrice   uint64        `json:"unitPrice"`
	UnitWindow  window.Window `json:"unitWindow"`
	BlockWindow window.Window `json:"blockWindow"`

	MinTxHght    uint64   `json:"minTxHeight"`
	ContainsWarp bool     `json:"containsWarp"`
	Txs          []ids.ID `json:"txs"`

	// TODO: migrate state root to be that of parent
	StateRoot     ids.ID `json:"stateRoot"`
	UnitsConsumed uint64 `json:"unitsConsumed"`
}

// Stateless is defined separately from "Block"
// in case external packages needs use the stateful block
// without mocking VM or parent block
type StatelessRootBlock struct {
	*RootBlock `json:"block"`

	id    ids.ID
	st    choices.Status
	t     time.Time
	bytes []byte

	bctx     *block.Context
	txBlocks []*StatelessTxBlock

	vm    VM
	state merkledb.TrieView
}

func NewGenesisBlock(root ids.ID, minUnit uint64, minBlock uint64) *RootBlock {
	return &RootBlock{
		UnitPrice:   minUnit,
		UnitWindow:  window.Window{},
		BlockWindow: window.Window{},

		StateRoot: root,
	}
}

func NewRootBlock(ectx *ExecutionContext, vm VM, parent snowman.Block, tmstp int64) *StatelessRootBlock {
	return &StatelessRootBlock{
		RootBlock: &RootBlock{
			Prnt:   parent.ID(),
			Tmstmp: tmstp,
			Hght:   parent.Height() + 1,

			UnitPrice:   ectx.NextUnitPrice,
			UnitWindow:  ectx.NextUnitWindow,
			BlockWindow: ectx.NextBlockWindow,
		},
		vm: vm,
		st: choices.Processing,
	}
}

func ParseStatelessRootBlock(
	ctx context.Context,
	source []byte,
	status choices.Status,
	vm VM,
) (*StatelessRootBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseRootBlock")
	defer span.End()

	blk, err := UnmarshalRootBlock(source, vm)
	if err != nil {
		return nil, err
	}
	// Not guaranteed that a parsed block is verified
	return ParseRootBlock(ctx, blk, source, status, vm)
}

func ParseRootBlock(
	ctx context.Context,
	blk *RootBlock,
	source []byte,
	status choices.Status,
	vm VM,
) (*StatelessRootBlock, error) {
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
		if len(blk.Txs) > r.GetMaxTxBlocks() {
			return nil, ErrBlockTooBig
		}
		// TODO: ensure aren't too many blocks in time period
	}

	if len(source) == 0 {
		nsource, err := blk.Marshal()
		if err != nil {
			return nil, err
		}
		source = nsource
	}
	b := &StatelessRootBlock{
		RootBlock: blk,
		t:         time.Unix(blk.Tmstmp, 0),
		bytes:     source,
		st:        status,
		vm:        vm,
		id:        utils.ToID(source),
	}

	// If we are parsing an older block, it will not be re-executed and should
	// not be tracked as a parsed block
	lastAccepted := b.vm.LastAcceptedBlock()
	if lastAccepted == nil || b.Hght <= lastAccepted.Hght { // nil when parsing genesis
		return b, nil
	}

	// Ensure we are tracking the block chunks we just parsed
	b.vm.RequestTxBlocks(ctx, b.MinTxHght, b.Txs)
	return b, nil
}

// [initializeBuilt] is invoked after a block is built
func (b *StatelessRootBlock) initializeBuilt(
	ctx context.Context,
	txBlocks []*StatelessTxBlock,
	state merkledb.TrieView,
) error {
	_, span := b.vm.Tracer().Start(ctx, "StatelessBlock.initializeBuilt")
	defer span.End()

	blk, err := b.RootBlock.Marshal()
	if err != nil {
		return err
	}
	b.bytes = blk
	b.id = utils.ToID(b.bytes)
	b.txBlocks = txBlocks
	b.state = state
	b.t = time.Unix(b.RootBlock.Tmstmp, 0)
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessRootBlock) ID() ids.ID { return b.id }

// implements "block.WithVerifyContext"
func (b *StatelessRootBlock) ShouldVerifyWithContext(context.Context) (bool, error) {
	return b.ContainsWarp, nil
}

// implements "block.WithVerifyContext"
func (b *StatelessRootBlock) VerifyWithContext(ctx context.Context, bctx *block.Context) error {
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
func (b *StatelessRootBlock) Verify(ctx context.Context) error {
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

func (b *StatelessRootBlock) verify(ctx context.Context, stateReady bool) error {
	// TODO: verify all chunks have right tmstp, unit price
	// TODO: verify all chunks have right pchainheight + contains warp
	// TODO: verify all chunks are done verifying

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
func (b *StatelessRootBlock) innerVerify(ctx context.Context) (merkledb.TrieView, error) {
	// Get state from final TxBlock execution
	state, err := b.vm.GetTxBlockState(ctx, b.Txs[len(b.Txs)-1])
	if err != nil {
		return nil, err
	}

	// Perform basic correctness checks before doing any expensive work
	var (
		log = b.vm.Logger()
		r   = b.vm.Rules(b.Tmstmp)
	)
	switch {
	case b.Timestamp().Unix() >= time.Now().Add(FutureBound).Unix():
		return nil, ErrTimestampTooLate
	case len(b.Txs) == 0:
		return nil, ErrNoTxs
	case len(b.Txs) > r.GetMaxTxBlocks():
		return nil, ErrBlockTooBig
	}

	// Verify parent is verified and available
	parent, err := b.vm.GetStatelessRootBlock(ctx, b.Prnt)
	if err != nil {
		log.Debug("could not get parent", zap.Stringer("id", b.Prnt))
		return nil, err
	}
	if b.Timestamp().Unix() < parent.Timestamp().Unix() {
		return nil, ErrTimestampTooEarly
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
	case b.BlockWindow != ectx.NextBlockWindow:
		return nil, ErrInvalidBlockWindow
	}
	log.Info(
		"verify context",
		zap.Uint64("height", b.Hght),
		zap.Uint64("unit price", b.UnitPrice),
	)

	// TODO: do root generation in final block and use height of inner block

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
	_, sspan := b.vm.Tracer().Start(ctx, "StatelessRootBlock.Verify.WaitSignatures")
	defer sspan.End()
	start = time.Now()
	for _, job := range b.txBlocks {
		if err := job.sigJob.Wait(); err != nil {
			return nil, err
		}
	}
	b.vm.RecordWaitSignatures(time.Since(start))
	return state, nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessRootBlock) Accept(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessRootBlock.Accept")
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
		// TODO: iterate through stateless tx blocks
		return errors.New("not implemented")
	}

	// Commit state if we don't return before here (would happen if we are still
	// syncing)
	if err := b.state.CommitToDB(ctx); err != nil {
		return err
	}

	// Set last accepted block
	return b.SetLastAccepted(ctx)
}

// SetLastAccepted is called during [Accept] and at the start and end of state
// sync.
func (b *StatelessRootBlock) SetLastAccepted(ctx context.Context) error {
	if err := b.vm.SetLastAccepted(b); err != nil {
		return err
	}
	b.st = choices.Accepted
	for _, txBlock := range b.txBlocks {
		if err := txBlock.Accept(ctx); err != nil {
			return err
		}
	}

	// [Accepted] will set in-memory variables needed to ensure we don't resync
	// all blocks when state sync finishes
	//
	// Note: We will not call [b.vm.Verified] before accepting during state sync
	b.vm.Accepted(ctx, b)
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessRootBlock) Reject(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessRootBlock.Reject")
	defer span.End()

	b.st = choices.Rejected
	for _, txBlock := range b.txBlocks {
		if err := txBlock.Reject(ctx); err != nil {
			return err
		}
	}
	b.vm.Rejected(ctx, b)
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessRootBlock) Status() choices.Status { return b.st }

// implements "snowman.Block"
func (b *StatelessRootBlock) Parent() ids.ID { return b.RootBlock.Prnt }

// implements "snowman.Block"
func (b *StatelessRootBlock) Bytes() []byte { return b.bytes }

// implements "snowman.Block"
func (b *StatelessRootBlock) Height() uint64 { return b.RootBlock.Hght }

// implements "snowman.Block"
func (b *StatelessRootBlock) Timestamp() time.Time { return b.t }

// State is used to verify txs in the mempool. It should never be written to.
//
// TODO: we should modify the interface here to only allow read-like messages
func (b *StatelessRootBlock) State() (Database, error) {
	if b.st == choices.Accepted {
		return b.vm.State()
	}
	if b.Processed() {
		return b.state, nil
	}
	return nil, ErrBlockNotProcessed
}

// Used to determine if should notify listeners and/or pass to controller
func (b *StatelessRootBlock) Processed() bool {
	return b.state != nil
}

// We assume this will only be called once we are done syncing, so it is safe
// to assume we will eventually get to a block with state.
func (b *StatelessRootBlock) childState(
	ctx context.Context,
	estimatedChanges int,
) (merkledb.TrieView, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessRootBlock.childState")
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
		return nil, errors.New("not implemented")
	}
	return b.state.NewPreallocatedView(estimatedChanges)
}

func (b *StatelessRootBlock) GetTxs() []ids.ID {
	return b.Txs
}

func (b *StatelessRootBlock) GetTxBlocks() []*StatelessTxBlock {
	return b.txBlocks
}

func (b *StatelessRootBlock) GetTimestamp() int64 {
	return b.Tmstmp
}

func (b *StatelessRootBlock) GetUnitPrice() uint64 {
	return b.UnitPrice
}

func (b *StatelessRootBlock) MaxTxHght() uint64 {
	l := len(b.Txs)
	if l == 0 {
		return b.MinTxHght
	}
	return b.MinTxHght + uint64(l-1)
}

func (b *RootBlock) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackUint64(b.UnitPrice)
	p.PackWindow(b.UnitWindow)
	p.PackWindow(b.BlockWindow)

	p.PackUint64(b.MinTxHght)
	p.PackBool(b.ContainsWarp)
	p.PackInt(len(b.Txs))
	for _, tx := range b.Txs {
		p.PackID(tx)
	}

	p.PackID(b.StateRoot)

	return p.Bytes(), p.Err()
}

func UnmarshalRootBlock(raw []byte, parser Parser) (*RootBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b RootBlock
	)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	b.UnitPrice = p.UnpackUint64(false)
	p.UnpackWindow(&b.UnitWindow)
	p.UnpackWindow(&b.BlockWindow)
	if err := p.Err(); err != nil {
		// Check that header was parsed properly before unwrapping transactions
		return nil, err
	}

	// Parse transactions
	b.MinTxHght = p.UnpackUint64(false)
	b.ContainsWarp = p.UnpackBool()
	txCount := p.UnpackInt(false) // could be 0 in genesis
	b.Txs = []ids.ID{}            // don't preallocate all to avoid DoS
	// TODO: check limit len here
	for i := 0; i < txCount; i++ {
		var txID ids.ID
		p.UnpackID(true, &txID)
		b.Txs = append(b.Txs, txID)
	}

	p.UnpackID(false, &b.StateRoot)

	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
	}
	return &b, p.Err()
}

type SyncableBlock struct {
	*StatelessRootBlock
}

func (sb *SyncableBlock) Accept(ctx context.Context) (block.StateSyncMode, error) {
	return sb.vm.AcceptedSyncableBlock(ctx, sb)
}

func NewSyncableBlock(sb *StatelessRootBlock) *SyncableBlock {
	return &SyncableBlock{sb}
}

func (sb *SyncableBlock) String() string {
	return fmt.Sprintf("%d:%s root=%s", sb.Height(), sb.ID(), sb.StateRoot)
}
