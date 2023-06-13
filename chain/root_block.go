package chain

import (
	"context"
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

	BlockWindow window.Window `json:"blockWindow"`

	MinTxHght    uint64   `json:"minTxHeight"`
	ContainsWarp bool     `json:"containsWarp"`
	Txs          []ids.ID `json:"txs"`

	// TODO: migrate state root to be that of parent
	StateRoot     ids.ID `json:"stateRoot"`
	UnitsConsumed uint64 `json:"unitsConsumed"`

	Issued int64 `json:"issued"`
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

	bctx *block.Context
	// These will only be populated if block was verified and still in cache
	txBlocks []*StatelessTxBlock

	firstVerify    time.Time
	recordedVerify bool

	vm VM
}

func NewGenesisRootBlock(txBlkID ids.ID, root ids.ID) *RootBlock {
	return &RootBlock{
		BlockWindow: window.Window{},
		Txs:         []ids.ID{txBlkID},
		StateRoot:   root,
	}
}

func NewRootBlock(ectx *RootExecutionContext, vm VM, parent snowman.Block, tmstp int64) *StatelessRootBlock {
	return &StatelessRootBlock{
		RootBlock: &RootBlock{
			Prnt:   parent.ID(),
			Tmstmp: tmstp,
			Hght:   parent.Height() + 1,

			BlockWindow: ectx.NextBlockWindow,
		},
		vm: vm,
		st: choices.Processing,
	}
}

func ParseStatelessRootBlock(
	ctx context.Context,
	txBlks []*StatelessTxBlock,
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
	return ParseRootBlock(ctx, blk, txBlks, source, status, vm)
}

func ParseRootBlock(
	ctx context.Context,
	blk *RootBlock,
	txBlks []*StatelessTxBlock,
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
		txBlocks:  txBlks,
	}

	// If we are parsing an older block, it will not be re-executed and should
	// not be tracked as a parsed block
	lastAccepted := b.vm.LastAcceptedBlock()
	if lastAccepted == nil || b.Hght <= lastAccepted.Hght { // nil when parsing genesis
		return b, nil
	}

	// Ensure we are tracking the block chunks we just parsed
	b.vm.RecordRootBlockIssuanceDiff(time.Since(time.UnixMilli(b.Issued)))
	b.vm.RecordTxBlocksMissing(b.vm.RequireTxBlocks(context.Background(), b.MinTxHght, b.Txs))
	return b, nil
}

// [initializeBuilt] is invoked after a block is built
func (b *StatelessRootBlock) initializeBuilt(
	ctx context.Context,
	txBlocks []*StatelessTxBlock,
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
			attribute.Bool("built", b.txBlockState() != nil),
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

func (b *StatelessRootBlock) Processed() bool {
	return b.txBlockState() != nil
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
		if err := b.innerVerify(ctx); err != nil {
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
func (b *StatelessRootBlock) innerVerify(ctx context.Context) error {
	if b.firstVerify.IsZero() {
		b.firstVerify = time.Now()
	}

	// Populate txBlocks
	txBlocks := make([]*StatelessTxBlock, len(b.Txs))
	var state merkledb.TrieView
	var containsWarp bool
	var unitsConsumed uint64
	for i, blkID := range b.Txs {
		blk, err := b.vm.GetStatelessTxBlock(ctx, blkID, b.MinTxHght+uint64(i))
		if err != nil {
			return err
		}
		if blk.Tmstmp != b.Tmstmp {
			// TODO: make block un-reverifiable
			return errors.New("invalid timestamp")
		}
		if b.ContainsWarp {
			if blk.PChainHeight != b.bctx.PChainHeight {
				// TODO: make block un-reverifiable
				return fmt.Errorf("invalid p-chain height with warp; found=%d context=%d", blk.PChainHeight, b.bctx.PChainHeight)
			}
		} else {
			if blk.PChainHeight != 0 {
				// TODO: make block un-reverifiable
				return fmt.Errorf("invalid p-chain height without warp; found=%d", blk.PChainHeight)
			}
		}
		if blk.ContainsWarp {
			containsWarp = true
		}
		if (blk.Last && blk.state == nil) || (!blk.Last && blk.processor == nil) {
			return errors.New("tx block state not ready")
		}
		unitsConsumed += blk.UnitsConsumed
		// Can't get from txBlockState because not populated yet
		state = blk.state
		txBlocks[i] = blk
	}
	if containsWarp != b.ContainsWarp {
		// TODO: make block un-reverifiable
		return errors.New("invalid warp status")
	}
	if !b.recordedVerify {
		b.vm.RecordVerifyWait(time.Since(b.firstVerify))
		b.recordedVerify = true
	}

	// Perform basic correctness checks before doing any expensive work
	var (
		log = b.vm.Logger()
		r   = b.vm.Rules(b.Tmstmp)
	)
	switch {
	case b.Timestamp().Unix() >= time.Now().Add(FutureBound).Unix():
		return ErrTimestampTooLate
	case len(b.Txs) == 0:
		return ErrNoTxs
	case len(b.Txs) > r.GetMaxTxBlocks():
		return ErrBlockTooBig
	case unitsConsumed != b.UnitsConsumed:
		return fmt.Errorf("%w: found=%d expected=%d", ErrInvalidUnitsConsumed, unitsConsumed, b.UnitsConsumed)
	}

	// Verify parent is verified and available
	parent, err := b.vm.GetStatelessRootBlock(ctx, b.Prnt)
	if err != nil {
		log.Debug("could not get parent", zap.Stringer("id", b.Prnt))
		return err
	}
	if b.Timestamp().Unix() < parent.Timestamp().Unix() {
		return ErrTimestampTooEarly
	}
	ectx, err := GenerateRootExecutionContext(ctx, b.vm.ChainID(), b.Tmstmp, parent, b.vm.Tracer(), r)
	if err != nil {
		return err
	}
	switch {
	case b.BlockWindow != ectx.NextBlockWindow:
		// TODO: make block un-reverifiable
		return ErrInvalidBlockWindow
	}

	// Root was already computed in TxBlock so this should return immediately
	computedRoot, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return err
	}
	if b.StateRoot != computedRoot {
		// TODO: make block un-reverifiable
		return fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			b.StateRoot,
		)
	}

	// Ensure signatures are verified
	if b.vm.GetVerifySignatures() {
		_, sspan := b.vm.Tracer().Start(ctx, "StatelessRootBlock.Verify.WaitSignatures")
		defer sspan.End()
		start := time.Now()
		for _, job := range txBlocks {
			if err := job.sigJob.Wait(); err != nil {
				return err
			}
		}
		b.vm.RecordWaitSignatures(time.Since(start))
	}
	b.txBlocks = txBlocks // only set once we know verification has passed
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessRootBlock) Accept(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessRootBlock.Accept")
	defer span.End()

	b.vm.RecordRootBlockAcceptanceDiff(time.Since(time.UnixMilli(b.Issued)))

	// Consider verifying the a block if it is not processed and we are no longer
	// syncing.
	state := b.txBlockState()
	if state == nil {
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
		// TODO: iterate through stateless tx blocks and verify
		return errors.New("not implemented")
	}

	// Commit state if we don't return before here (would happen if we are still
	// syncing)
	start := time.Now()
	if err := state.CommitToDB(ctx); err != nil {
		return err
	}
	b.vm.RecordCommitState(time.Since(start))

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
	if state := b.txBlockState(); state != nil {
		return state, nil
	}
	return nil, ErrBlockNotProcessed
}

func (b *StatelessRootBlock) LastTxBlock() (*StatelessTxBlock, error) {
	l := len(b.txBlocks)
	if l > 0 {
		return b.txBlocks[l-1], nil
	}
	lid := b.Txs[len(b.Txs)-1]
	// 10 + [10, 11, 12, 13]
	return b.vm.GetStatelessTxBlock(context.TODO(), lid, b.MinTxHght+uint64(len(b.Txs)-1))
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

func (b *StatelessRootBlock) MaxTxHght() uint64 {
	l := len(b.Txs)
	if l == 0 {
		return b.MinTxHght
	}
	// 10 + [10,11,12,13]
	return b.MinTxHght + uint64(l-1)
}

func (b *StatelessRootBlock) txBlockState() merkledb.TrieView {
	l := len(b.txBlocks)
	if l == 0 {
		return nil
	}
	// TODO: handle case where empty during state sync
	return b.txBlocks[l-1].state
}

func (b *RootBlock) Marshal() ([]byte, error) {
	size := consts.IDLen + consts.Uint64Len + consts.Uint64Len + window.WindowSliceSize +
		consts.Uint64Len + codec.BoolLen + consts.IntLen + len(b.Txs)*consts.IDLen + consts.IDLen +
		consts.Uint64Len + consts.Uint64Len
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackWindow(b.BlockWindow)

	p.PackUint64(b.MinTxHght)
	p.PackBool(b.ContainsWarp)
	p.PackInt(len(b.Txs))
	for _, tx := range b.Txs {
		p.PackID(tx)
	}

	p.PackID(b.StateRoot)
	p.PackUint64(b.UnitsConsumed)
	p.PackInt64(b.Issued)

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

	p.UnpackWindow(&b.BlockWindow)

	b.MinTxHght = p.UnpackUint64(false)
	b.ContainsWarp = p.UnpackBool()
	if err := p.Err(); err != nil {
		// Check that header was parsed properly before unwrapping transactions
		return nil, err
	}

	// Parse transactions
	txCount := p.UnpackInt(false) // could be 0 in genesis
	b.Txs = []ids.ID{}            // don't preallocate all to avoid DoS
	// TODO: check limit len here
	for i := 0; i < txCount; i++ {
		var txID ids.ID
		p.UnpackID(true, &txID)
		b.Txs = append(b.Txs, txID)
	}

	p.UnpackID(false, &b.StateRoot)
	b.UnitsConsumed = p.UnpackUint64(false) // could be 0 in genesis
	b.Issued = p.UnpackInt64(false)

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
