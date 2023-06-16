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
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
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

	MinTxHght uint64   `json:"minTxHeight"`
	TxBlocks  []ids.ID `json:"txBlocks"`

	ContainsWarp bool `json:"containsWarp"`

	// TEMP
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
	txBlocks     []*StatelessTxBlock
	state        merkledb.TrieView
	warpMessages map[ids.ID]*warpJob
	vdrState     validators.State
	results      []*Result

	firstVerify    time.Time
	recordedVerify bool

	vm VM
}

func NewGenesisRootBlock(txBlkID ids.ID) *RootBlock {
	return &RootBlock{
		TxBlocks: []ids.ID{txBlkID},
	}
}

func NewRootBlock(ectx *RootExecutionContext, vm VM, parent snowman.Block, tmstp int64) *StatelessRootBlock {
	return &StatelessRootBlock{
		RootBlock: &RootBlock{
			Prnt:   parent.ID(),
			Tmstmp: tmstp,
			Hght:   parent.Height() + 1,
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
		if len(blk.TxBlocks) == 0 {
			return nil, ErrNoTxs
		}
		r := vm.Rules(blk.Tmstmp)
		if len(blk.TxBlocks) > r.GetMaxTxBlocks() {
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
	b.vm.RecordTxBlocksMissing(b.vm.RequireTxBlocks(context.Background(), b.MinTxHght, b.TxBlocks))
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
			attribute.Int("txs", len(b.TxBlocks)),
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
			attribute.Int("txs", len(b.TxBlocks)),
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
	txBlocks := make([]*StatelessTxBlock, len(b.TxBlocks))
	var state merkledb.TrieView
	var containsWarp bool
	var unitsConsumed uint64
	for i, blkID := range b.TxBlocks {
		blk, err := b.vm.GetStatelessTxBlock(ctx, blkID, b.MinTxHght+uint64(i))
		if err != nil {
			// TODO: stopgap that should be removed
			b.vm.Logger().Warn("missing tx block when starting verify", zap.Stringer("blkID", blkID))
			b.vm.RetryVerify(ctx, b.TxBlocks)
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
	case len(b.TxBlocks) == 0:
		return ErrNoTxs
	case len(b.TxBlocks) > r.GetMaxTxBlocks():
		return ErrBlockTooBig
	}

	// Verify parent is available
	parent, err := b.vm.GetStatelessRootBlock(ctx, b.Prnt)
	if err != nil {
		log.Debug("could not get parent", zap.Stringer("id", b.Prnt))
		return err
	}
	if b.Timestamp().Unix() < parent.Timestamp().Unix() {
		return ErrTimestampTooEarly
	}

	// // Root was already computed in TxBlock so this should return immediately
	// computedRoot, err := state.GetMerkleRoot(ctx)
	// if err != nil {
	// 	return err
	// }
	// if b.StateRoot != computedRoot {
	// 	// TODO: make block un-reverifiable
	// 	return fmt.Errorf(
	// 		"%w: expected=%s found=%s",
	// 		ErrStateRootMismatch,
	// 		computedRoot,
	// 		b.StateRoot,
	// 	)
	// }

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
		// // The state of this block was not calculated during the call to
		// // [StatelessBlock.Verify]. This is because the VM was state syncing
		// // and did not have the state necessary to verify the block.
		// updated, err := b.vm.UpdateSyncTarget(b)
		// if err != nil {
		// 	return err
		// }
		// if updated {
		// 	b.vm.Logger().
		// 		Info("updated state sync target", zap.Stringer("id", b.ID()), zap.Stringer("root", b.StateRoot))
		// 	return nil // the sync is still ongoing
		// }
		// TODO: iterate through stateless tx blocks and verify
		return errors.New("not implemented")
	}

	// Commit state if we don't return before here (would happen if we are still
	// syncing)
	// start := time.Now()
	// if err := state.CommitToDB(ctx); err != nil {
	// 	return err
	// }
	// b.vm.RecordCommitState(time.Since(start))

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

// warpJob is used to signal to a listner that a *warp.Message has been
// verified.
type warpJob struct {
	msg          *warp.Message
	signers      int
	verifiedChan chan bool
	verified     bool
	warpNum      int
}

func extraVerify() {
	b.warpMessages = map[ids.ID]*warpJob{}
	
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

