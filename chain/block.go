// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ snowman.Block           = &StatelessBlock{}
	_ block.WithVerifyContext = &StatelessBlock{}
	_ block.StateSummary      = &SyncableBlock{}
)

type StatefulBlock struct {
	Parent     ids.ID `json:"parent"`
	Height     uint64 `json:"height"`
	Timestamp  int64  `json:"timestamp"`
	HasContext bool   `json:"hasContext"`

	// AvailableChunks is a collection of valid Chunks that will be executed in
	// the future.
	AvailableChunks []*ChunkCertificate `json:"availableChunks"`

	// StartRoot is the root of the post-execution state
	// of [Parent].
	//
	// This "deferred root" design allows for merklization
	// to be done asynchronously instead of during [Build]
	// or [Verify], which reduces the amount of time we are
	// blocking the consensus engine from voting on the block,
	// starting the verification of another block, etc.
	StartRoot      ids.ID   `json:"startRoot"`
	ExecutedChunks []ids.ID `json:"executedChunks"`

	built bool
}

func (b *StatefulBlock) Size() int {
	return consts.IDLen + consts.Uint64Len + consts.Int64Len + consts.BoolLen +
		consts.IntLen + codec.CummSize(b.AvailableChunks) +
		consts.IDLen + consts.IntLen + len(b.ExecutedChunks)*consts.IDLen
}

func (b *StatefulBlock) ID() (ids.ID, error) {
	blk, err := b.Marshal()
	if err != nil {
		return ids.ID{}, err
	}
	return utils.ToID(blk), nil
}

func (b *StatefulBlock) Marshal() ([]byte, error) {
	p := codec.NewWriter(b.Size(), consts.NetworkSizeLimit)

	p.PackID(b.Parent)
	p.PackUint64(b.Height)
	p.PackInt64(b.Timestamp)
	p.PackBool(b.HasContext)

	p.PackInt(len(b.AvailableChunks))
	for _, cert := range b.AvailableChunks {
		if err := cert.MarshalPacker(p); err != nil {
			return nil, err
		}
	}

	p.PackID(b.StartRoot)
	p.PackInt(len(b.ExecutedChunks))
	for _, chunk := range b.ExecutedChunks {
		p.PackID(chunk)
	}

	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return bytes, nil
}

func UnmarshalBlock(raw []byte) (*StatefulBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b StatefulBlock
	)

	p.UnpackID(false, &b.Parent)
	b.Height = p.UnpackUint64(false)
	b.Timestamp = p.UnpackInt64(false)
	b.HasContext = p.UnpackBool()

	// Parse available chunks
	chunkCount := p.UnpackInt(false)          // can produce empty blocks
	b.AvailableChunks = []*ChunkCertificate{} // don't preallocate all to avoid DoS
	for i := 0; i < chunkCount; i++ {
		cert, err := UnmarshalChunkCertificatePacker(p)
		if err != nil {
			return nil, err
		}
		b.AvailableChunks = append(b.AvailableChunks, cert)
	}

	// Parse executed chunks
	p.UnpackID(false, &b.StartRoot)
	chunkCount = p.UnpackInt(false) // can produce empty blocks
	b.ExecutedChunks = []ids.ID{}   // don't preallocate all to avoid DoS
	for i := 0; i < chunkCount; i++ {
		var id ids.ID
		p.UnpackID(true, &id)
		b.ExecutedChunks = append(b.ExecutedChunks, id)
	}

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &b, p.Err()
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
		Timestamp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),

		// StartRoot should include all allocates made when loading the genesis file
		StartRoot: root,
	}
}

// Stateless is defined separately from "Block"
// in case external packages needs use the stateful block
// without mocking VM or parent block
type StatelessBlock struct {
	*StatefulBlock `json:"block"`

	vm VM

	id    ids.ID
	st    choices.Status
	t     time.Time
	bytes []byte

	parent     *StatelessBlock
	execHeight *uint64

	chunks set.Set[ids.ID]

	bctx *block.Context
}

func NewBlock(vm VM, parent snowman.Block, tmstp int64, context bool) *StatelessBlock {
	return &StatelessBlock{
		StatefulBlock: &StatefulBlock{
			Parent:     parent.ID(),
			Timestamp:  tmstp,
			Height:     parent.Height() + 1,
			HasContext: context,
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

	blk, err := UnmarshalBlock(source)
	if err != nil {
		return nil, err
	}
	// Not guaranteed that a parsed block is verified
	return ParseStatefulBlock(ctx, blk, source, status, vm)
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
	if blk.Timestamp > time.Now().Add(FutureBound).UnixMilli() {
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
		t:             time.UnixMilli(blk.Timestamp),
		bytes:         source,
		st:            status,
		vm:            vm,
		id:            utils.ToID(source),
	}

	// If we are parsing an older block, it will not be re-executed and should
	// not be tracked as a parsed block
	lastAccepted := b.vm.LastAcceptedBlock()
	if lastAccepted == nil || blk.Height <= lastAccepted.StatefulBlock.Height { // nil when parsing genesis
		return b, nil
	}

	// Update set (handle this on built)
	b.chunks = set.NewSet[ids.ID](len(blk.AvailableChunks))
	for _, cert := range blk.AvailableChunks {
		b.chunks.Add(cert.Chunk)
	}

	// TODO: add parent, execHeight, and bctx to the block?

	return b, nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) ID() ids.ID { return b.id }

// implements "block.WithVerifyContext"
func (b *StatelessBlock) ShouldVerifyWithContext(context.Context) (bool, error) {
	// If we build with context, we should verify with context. We may use this context
	// to update the P-Chain height for the next epoch.
	return b.HasContext, nil
}

// implements "block.WithVerifyContext"
func (b *StatelessBlock) VerifyWithContext(ctx context.Context, bctx *block.Context) error {
	start := time.Now()
	defer func() {
		b.vm.RecordBlockVerify(time.Since(start))
	}()

	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessBlock.VerifyWithContext",
		oteltrace.WithAttributes(
			attribute.Int64("height", int64(b.StatefulBlock.Height)),
			attribute.Int64("pchainHeight", int64(bctx.PChainHeight)),
		),
	)
	defer span.End()

	// Persist the context in case we need it during Accept
	b.bctx = bctx

	// Proceed with normal verification
	err := b.verify(ctx)
	if err != nil {
		b.vm.Logger().Error(
			"verification failed",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Stringer("parentID", b.Parent()),
			zap.Error(err),
		)
	}
	return err
}

// implements "snowman.Block"
func (b *StatelessBlock) Verify(ctx context.Context) error {
	start := time.Now()
	defer func() {
		b.vm.RecordBlockVerify(time.Since(start))
	}()

	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessBlock.Verify",
		oteltrace.WithAttributes(
			attribute.Int64("height", int64(b.StatefulBlock.Height)),
		),
	)
	defer span.End()

	err := b.verify(ctx)
	if err != nil {
		b.vm.Logger().Error(
			"verification failed",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Stringer("parentID", b.Parent()),
			zap.Error(err),
		)
	}
	return err
}

// Tasks
// 1) verify certificates (correct signatures, correct weight, not duplicates)
// 2) verify parent state root is correct
// 3) verify executed certificates (correct IDs)
func (b *StatelessBlock) verify(ctx context.Context) error {
	// Skip verification if we built this block
	if b.built {
		b.vm.Logger().Info(
			"skipping verify because built block",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Stringer("parentID", b.Parent()),
		)
		b.vm.Verified(ctx, b)
		return nil
	}

	// TODO: skip verification if state does not exist yet

	var (
		log   = b.vm.Logger()
		r     = b.vm.Rules(b.StatefulBlock.Timestamp)
		epoch = utils.Epoch(b.StatefulBlock.Timestamp, r.GetEpochDuration())
	)

	// Fetch P-Chain height for epoch from executed state
	//
	// If missing and state read has timestamp updated in same or previous slot, we know that epoch
	// cannot be set.
	timestampKey := HeightKey(b.vm.StateManager().TimestampKey())
	epochKey := EpochKey(b.vm.StateManager().EpochKey(epoch))
	nextEpochKey := EpochKey(b.vm.StateManager().EpochKey(epoch + 1))
	values, errs := b.vm.Engine().ReadLatestState(ctx, [][]byte{timestampKey, epochKey, nextEpochKey})
	if errs[0] != nil {
		return fmt.Errorf("%w: can't read timestamp key", errs[0])
	}
	executedTime := int64(binary.BigEndian.Uint64(values[0]))
	executedEpoch := utils.Epoch(executedTime, r.GetEpochDuration())
	if executedEpoch+2 < epoch {
		return errors.New("executed tip is too far behind to verify block")
	}
	var pchainHeight *uint64
	if errs[1] == nil {
		height := binary.BigEndian.Uint64(values[1])
		pchainHeight = &height
	} else {
		log.Warn("missing epoch key", zap.Uint64("epoch", epoch))
	}
	var nextPchainHeight *uint64
	if errs[2] == nil {
		height := binary.BigEndian.Uint64(values[2])
		nextPchainHeight = &height
	} else {
		log.Warn("missing epoch key", zap.Uint64("epoch", epoch+1))
	}

	// Perform basic correctness checks before doing any expensive work
	if b.Timestamp().UnixMilli() > time.Now().Add(FutureBound).UnixMilli() {
		return ErrTimestampTooLate
	}

	// Check that gap between parent is at least minimum
	//
	// We do not have access to state here, so we must use the parent block.
	parent, err := b.vm.GetStatelessBlock(ctx, b.StatefulBlock.Parent)
	if err != nil {
		log.Error("block verification failed, missing parent", zap.Stringer("parentID", b.StatefulBlock.Parent), zap.Error(err))
		return fmt.Errorf("%w: can't get parent block %s", err, b.StatefulBlock.Parent)
	}
	if b.StatefulBlock.Height != parent.StatefulBlock.Height+1 {
		return ErrInvalidBlockHeight
	}
	b.parent = parent
	parentTimestamp := parent.StatefulBlock.Timestamp
	if b.StatefulBlock.Timestamp < parentTimestamp+r.GetMinBlockGap() {
		return ErrTimestampTooEarly
	}

	// Check duplicate certificates
	repeats, err := parent.IsRepeatChunk(ctx, b.AvailableChunks, set.NewBits())
	if err != nil {
		return fmt.Errorf("%w: can't check if chunk is repeat", err)
	}
	if repeats.Len() > 0 {
		return errors.New("duplicate chunk issuance")
	}

	// Verify certificates
	if pchainHeight != nil {
		// We get the validator set once and reuse it instead of using warp verify (which generates the set each time).

		// Get validator set at current height
		//
		// TODO: get from validator manager
		vdrSet, err := b.vm.ValidatorState().GetValidatorSet(ctx, *pchainHeight, b.vm.SubnetID())
		if err != nil {
			return fmt.Errorf("%w: can't get validator set", err)
		}
		vdrList, totalWeight, err := utils.ConstructCanonicalValidatorSet(vdrSet)
		if err != nil {
			return fmt.Errorf("%w: can't construct canonical validator set", err)
		}

		// Verify certificates
		//
		// TODO: make parallel
		// TODO: cache verifications (may be verified multiple times at the same p-chain height while
		// waiting for execution to complete).
		for i, cert := range b.AvailableChunks {
			// TODO: Ensure cert is from a validator
			//
			// Signers verify that validator signed chunk with right key when signing chunk, so may
			// not be necessary?
			//
			// TODO: consider not verifying this in case validator set changes?

			// TODO: consider moving to chunk verify

			// Ensure chunk is not expired
			if cert.Slot < b.StatefulBlock.Timestamp {
				return ErrTimestampTooLate
			}

			// Ensure chunk is not too early
			//
			// TODO: ensure slot is in the block epoch
			if cert.Slot > b.StatefulBlock.Timestamp+r.GetValidityWindow() {
				return ErrTimestampTooEarly
			}

			// Ensure chunk expiry is aligned to a tenth of a second
			//
			// Signatures will only be given for a configurable number of chunks per
			// second.
			//
			// TODO: consider moving to unmarshal
			if cert.Slot%consts.MillisecondsPerDecisecond != 0 {
				return ErrMisalignedTime
			}

			// Verify multi-signature
			filteredVdrs, err := warp.FilterValidators(cert.Signers, vdrList)
			if err != nil {
				return err
			}
			filteredWeight, err := warp.SumWeight(filteredVdrs)
			if err != nil {
				return err
			}
			// TODO: make this a const
			if err := warp.VerifyWeight(filteredWeight, totalWeight, 67, 100); err != nil {
				return err
			}
			aggrPubKey, err := warp.AggregatePublicKeys(filteredVdrs)
			if err != nil {
				return err
			}
			if !cert.VerifySignature(r.NetworkID(), r.ChainID(), aggrPubKey) {
				return fmt.Errorf("%w: pk=%s signature=%s blkCtx=%d cert=%d certID=%s signers=%d filteredWeight=%d totalWeight=%d", errors.New("certificate invalid"), hex.EncodeToString(bls.PublicKeyToBytes(aggrPubKey)), hex.EncodeToString(bls.SignatureToBytes(cert.Signature)), *&b.bctx.PChainHeight, i, cert.Chunk, len(filteredVdrs), filteredWeight, totalWeight)
			}
		}
	} else {
		if len(b.AvailableChunks) > 0 {
			return errors.New("block context not provided but have available chunks")
		}
	}

	// Verify start root and execution results
	depth := r.GetBlockExecutionDepth()
	if b.StatefulBlock.Height <= depth {
		if b.StartRoot != ids.Empty || len(b.ExecutedChunks) > 0 {
			return errors.New("no execution result should exist")
		}
	} else {
		execHeight := b.StatefulBlock.Height - depth
		root, executed, err := b.vm.Engine().Results(execHeight)
		if err != nil {
			// TODO: add stat for tracking how often this happens
			// TODOL handle case where we state synced and don't have results
			log.Warn("could not get results for block", zap.Uint64("height", execHeight))
			return fmt.Errorf("%w: no results for execHeight", err)
		}
		if b.StartRoot != root {
			return errors.New("start root mismatch")
		}
		if len(b.ExecutedChunks) != len(executed) {
			return errors.New("executed chunks count mismatch")
		}
		for i, id := range b.ExecutedChunks {
			if id != executed[i] {
				return errors.New("executed chunks mismatch")
			}
		}
		b.execHeight = &execHeight
	}
	b.vm.Verified(ctx, b)

	if b.execHeight == nil {
		// TODO: create a stringer for *Uint64
		log.Info(
			"verified block",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Stringer("parentID", b.Parent()),
			zap.Int("available chunks", len(b.AvailableChunks)),
			zap.Stringer("start root", b.StartRoot),
			zap.Int("executed chunks", len(b.ExecutedChunks)),
		)
	} else {
		log.Info(
			"verified block",
			zap.Stringer("blockID", b.ID()),
			zap.Uint64("height", b.StatefulBlock.Height),
			zap.Uint64("execHeight", *b.execHeight),
			zap.Stringer("parentID", b.Parent()),
			zap.Int("available chunks", len(b.AvailableChunks)),
			zap.Stringer("start root", b.StartRoot),
			zap.Int("executed chunks", len(b.ExecutedChunks)),
		)
	}
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

	b.st = choices.Accepted

	// Start async execution
	if b.StatefulBlock.Height > 0 { // nothing to execute in genesis
		b.vm.Engine().Execute(b)
	}

	// Collect async results (if any)
	var (
		filteredChunks []*FilteredChunk
		feeManager     *FeeManager
	)
	if b.execHeight != nil {
		fm, _, fc, err := b.vm.Engine().Commit(ctx, *b.execHeight)
		if err != nil {
			return fmt.Errorf("%w: cannot commit", err)
		}
		filteredChunks = fc
		feeManager = fm
	}
	b.vm.Accepted(ctx, b, feeManager, filteredChunks)
	b.vm.Logger().Info(
		"accept returned",
		zap.Uint64("height", b.StatefulBlock.Height),
		zap.Stringer("blockID", b.ID()),
	)
	return nil
}

// used for accepting syncable blocks
func (b *StatelessBlock) MarkAccepted(ctx context.Context) {
	// Accept block and free unnecessary memory
	b.st = choices.Accepted

	// [Accepted] will persist the block to disk and set in-memory variables
	// needed to ensure we don't resync all blocks when state sync finishes.
	//
	// Note: We will not call [b.vm.Verified] before accepting during state sync
	// TODO: this is definitely wrong, we should fix it
	b.vm.Accepted(ctx, b, nil, nil)
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
func (b *StatelessBlock) Parent() ids.ID { return b.StatefulBlock.Parent }

// implements "snowman.Block"
func (b *StatelessBlock) Bytes() []byte { return b.bytes }

// implements "snowman.Block"
func (b *StatelessBlock) Height() uint64 { return b.StatefulBlock.Height }

// implements "snowman.Block"
func (b *StatelessBlock) Timestamp() time.Time { return b.t }

func (b *StatelessBlock) IsRepeatChunk(ctx context.Context, certs []*ChunkCertificate, marker set.Bits) (set.Bits, error) {
	if b.st == choices.Accepted || b.StatefulBlock.Height == 0 /* genesis */ {
		return b.vm.IsRepeatChunk(ctx, certs, marker), nil
	}
	for i, cert := range certs {
		if marker.Contains(i) {
			continue
		}
		if b.chunks.Contains(cert.Chunk) {
			marker.Add(i)
		}
	}
	parent, err := b.vm.GetStatelessBlock(ctx, b.StatefulBlock.Parent)
	if err != nil {
		return marker, err
	}
	return parent.IsRepeatChunk(ctx, certs, marker)
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
	return fmt.Sprintf("%d:%s root=%s", sb.Height(), sb.ID(), sb.StartRoot)
}
