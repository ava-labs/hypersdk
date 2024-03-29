// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
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
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

// We set the genesis block timestamp to be after the ProposerVM fork activation.
//
// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
// where the child timestamp is rounded down to the nearest second (which may be before
// the timestamp of its parent, which is denoted in milliseconds).
//
// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
// .../vms/proposervm/pre_fork_block.go#L201
var GenesisTime = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

var (
	_ snowman.Block      = &StatelessBlock{}
	_ block.StateSummary = &SyncableBlock{}
)

type StatefulBlock struct {
	PHeight uint64 `json:"pHeight"` // 0 means no context

	Parent    ids.ID `json:"parent"`
	Height    uint64 `json:"height"`
	Timestamp int64  `json:"timestamp"`

	// AvailableChunks is a collection of valid Chunks that will be executed in
	// the future.
	AvailableChunks []*ChunkCertificate `json:"availableChunks"`

	ExecutedChunks []ids.ID `json:"executedChunks"`
	Root           ids.ID   `json:"root"`

	built bool
}

func (b *StatefulBlock) Size() int {
	return consts.Uint64Len + consts.IDLen + consts.Uint64Len + consts.Int64Len +
		consts.IntLen + codec.CummSize(b.AvailableChunks) +
		consts.IntLen + len(b.ExecutedChunks)*consts.IDLen + consts.IDLen
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

	p.PackUint64(b.PHeight)

	p.PackID(b.Parent)
	p.PackUint64(b.Height)
	p.PackInt64(b.Timestamp)

	p.PackInt(len(b.AvailableChunks))
	for _, cert := range b.AvailableChunks {
		if err := cert.MarshalPacker(p); err != nil {
			return nil, err
		}
	}

	p.PackInt(len(b.ExecutedChunks))
	for _, chunk := range b.ExecutedChunks {
		p.PackID(chunk)
	}
	p.PackID(b.Root)

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

	b.PHeight = p.UnpackUint64(false) // 0 when building without context

	p.UnpackID(false, &b.Parent)
	b.Height = p.UnpackUint64(false)
	b.Timestamp = p.UnpackInt64(false)

	// Parse available chunks
	availableChunks := p.UnpackInt(false)                // can produce empty blocks
	b.AvailableChunks = make([]*ChunkCertificate, 0, 16) // don't preallocate all to avoid DoS
	seen := set.NewSet[ids.ID](16)                       // TODO: make prealloc a config
	for i := 0; i < availableChunks; i++ {
		cert, err := UnmarshalChunkCertificatePacker(p)
		if err != nil {
			return nil, err
		}
		b.AvailableChunks = append(b.AvailableChunks, cert)
		if seen.Contains(cert.Chunk) {
			return nil, fmt.Errorf("duplicate chunk %s in block %d", cert.Chunk, b.Height)
		}
		seen.Add(cert.Chunk)
	}

	// Parse executed chunks
	executedChunks := p.UnpackInt(false) // can produce empty blocks
	b.ExecutedChunks = []ids.ID{}        // don't preallocate all to avoid DoS
	for i := 0; i < executedChunks; i++ {
		var id ids.ID
		p.UnpackID(true, &id)
		b.ExecutedChunks = append(b.ExecutedChunks, id)
	}
	p.UnpackID(false, &b.Root) // only some roots are populated

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: message=%d remaining=%d extra=%x", ErrInvalidObject, len(raw), len(raw)-p.Offset(), raw[p.Offset():])
	}
	return &b, p.Err()
}

func NewGenesisBlock(root ids.ID) *StatefulBlock {
	return &StatefulBlock{
		Timestamp: GenesisTime,
		Root:      root,
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

	verifiedCerts bool

	parent     *StatelessBlock
	rootHeight *uint64
	execHeight *uint64

	chunks set.Set[ids.ID]
}

func NewBlock(vm VM, pHeight uint64, parent snowman.Block, tmstp int64) *StatelessBlock {
	return &StatelessBlock{
		StatefulBlock: &StatefulBlock{
			PHeight:   pHeight,
			Parent:    parent.ID(),
			Timestamp: tmstp,
			Height:    parent.Height() + 1,
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
	if blk.Timestamp > time.Now().UnixMilli()+consts.ClockSkewAllowance {
		return nil, ErrTimestampTooEarly
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
	return b, nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessBlock) ID() ids.ID { return b.id }

// implements "snowman.Block"
//
// Tasks
// 1) verify certificates (correct signatures, correct weight, not duplicates)
// 2) verify parent state root is correct
// 3) verify executed certificates (correct IDs)
func (b *StatelessBlock) Verify(ctx context.Context) error {
	start := time.Now()
	success := false
	defer func() {
		if success {
			b.vm.RecordBlockVerify(time.Since(start))
		} else {
			// Can happen if block results are not yet ready
			b.vm.RecordBlockVerifyFail()
		}
	}()

	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessBlock.Verify",
		oteltrace.WithAttributes(
			attribute.Int64("height", int64(b.StatefulBlock.Height)),
		),
	)
	defer span.End()

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

	// Ensure p-chain height referenced is valid
	var (
		log   = b.vm.Logger()
		r     = b.vm.Rules(b.StatefulBlock.Timestamp)
		epoch = utils.Epoch(b.StatefulBlock.Timestamp, r.GetEpochDuration())
	)
	if !b.verifiedCerts {
		validHeight, err := b.vm.IsValidHeight(ctx, b.PHeight)
		if err != nil {
			return fmt.Errorf("%w: can't determine if valid height", err)
		}
		if !validHeight {
			return fmt.Errorf("invalid p-chain height: %d", b.PHeight)
		}

		// Ensure no certificates if P-Chain height is 0
		//
		// Even if there is an epoch, this height is used to verify warp
		// messages.
		if b.PHeight == 0 && len(b.AvailableChunks) > 0 {
			return errors.New("no certificates should exist in a block without context")
		}

		// TODO: skip verification if state does not exist yet (state sync)

		// Fetch P-Chain height for epoch from executed state
		//
		// If missing and state read has timestamp updated in same or previous slot, we know that epoch
		// cannot be set.
		timestamp, heights, err := b.vm.Engine().GetEpochHeights(ctx, []uint64{epoch, epoch + 1})
		if err != nil {
			return fmt.Errorf("%w: can't get epoch heights", err)
		}
		executedEpoch := utils.Epoch(timestamp, r.GetEpochDuration())
		if executedEpoch+1 < epoch && len(b.AvailableChunks) > 0 { // if execution in epoch 2 while trying to verify 4 and 5, we need to wait (should be rare)
			log.Error(
				"executed tip is too far behind to verify block with certs",
				zap.Uint64("executedEpoch", executedEpoch),
				zap.Uint64("epoch", epoch),
				zap.Stringer("blockID", b.ID()),
			)
			return errors.New("executed tip is too far behind to verify block with certs")
		}
		// We allow verfication to proceed if no available chunks and no epochs stored so that epochs could be set.

		// Perform basic correctness checks before doing any expensive work
		if b.Timestamp().UnixMilli() > time.Now().UnixMilli()+consts.ClockSkewAllowance {
			return ErrTimestampTooEarly
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
			log.Error("block contains duplicate chunk")
			return errors.New("duplicate chunk issuance")
		}

		// Verify certificates
		//
		// TODO: make parallel
		// TODO: cache verifications (may be verified multiple times at the same p-chain height while
		// waiting for execution to complete).
		for i, cert := range b.AvailableChunks {
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

			// Get validator set for the epoch
			certEpoch := utils.Epoch(cert.Slot, r.GetEpochDuration())
			heightIndex := certEpoch - epoch
			if heights[heightIndex] == nil {
				log.Error(
					"certificate is from missing epoch",
					zap.Uint64("epoch", certEpoch),
					zap.Stringer("chunkID", cert.Chunk),
				)
				return errors.New("missing epoch")
			}

			// Get the public key for the signers
			aggrPubKey, err := b.vm.GetAggregatePublicKey(ctx, *heights[heightIndex], cert.Signers, 67, 100) // TODO: add consts
			if err != nil {
				return fmt.Errorf("%w: can't generate aggregate public key", err)
			}
			if !cert.VerifySignature(r.NetworkID(), r.ChainID(), aggrPubKey) {
				log.Error(
					"certificate has invalid signature",
					zap.Stringer("blockID", b.ID()),
					zap.Stringer("chunkID", cert.Chunk),
				)
				return fmt.Errorf("%w: pk=%s signature=%s pHeight=%d cert=%d certID=%s signers=%s", errors.New("certificate invalid"), hex.EncodeToString(bls.PublicKeyToCompressedBytes(aggrPubKey)), hex.EncodeToString(bls.SignatureToBytes(cert.Signature)), b.PHeight, i, cert.Chunk, cert.Signers.String())
			}
		}

		// If we get this far, record we've already verified the certificates in this block so we don't do it again if
		// block results aren't ready yet. We don't just check results first because we want to give as much time as
		// possible for root generation to complete (and can do meaningful work here).
		b.verifiedCerts = true
	}

	// Verify execution results
	depth := r.GetBlockExecutionDepth()
	if b.StatefulBlock.Height <= depth {
		if len(b.ExecutedChunks) > 0 {
			return errors.New("no execution result should exist")
		}
	} else {
		var (
			execHeight = b.StatefulBlock.Height - depth
			executed   []ids.ID
			err        error
		)
		for {
			executed, err = b.vm.Engine().Results(execHeight)
			if err != nil {
				// TODO: handle case where we state synced and don't have results
				if b.vm.IsBootstrapped() {
					log.Debug("could not get results for block", zap.Uint64("height", execHeight))
					return fmt.Errorf("%w: no results for execHeight", err)
				}
				// If we haven't finished bootstrapping, we can't fail.
				//
				// TODO: we should actually not verify any of this (long p-chain lookbacks)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			break
		}
		if len(b.ExecutedChunks) != len(executed) {
			log.Error("block has invalid executed chunks count", zap.Stringer("blockID", b.ID()), zap.Int("expectedCount", len(executed)), zap.Int("actualCount", len(b.ExecutedChunks)))
			panic("executed chunk count mismatch") // could help us debug
		}
		for i, id := range b.ExecutedChunks {
			if id != executed[i] {
				log.Error("block has invalid executed chunks", zap.Stringer("blockID", b.ID()), zap.Int("index", i), zap.Stringer("expectedID", executed[i]), zap.Stringer("actualID", id))
				panic("executed chunk mismatch") // could help us debug
			}
		}
		b.execHeight = &execHeight
	}

	// Verify roots
	frequency := r.GetRootFrequency()
	if b.StatefulBlock.Height > frequency && b.StatefulBlock.Height%frequency == 0 {
		var (
			rootHeight = b.StatefulBlock.Height - frequency
			root       ids.ID
			err        error
		)
		for {
			root, err = b.vm.Engine().Root(rootHeight)
			if err != nil {
				if b.vm.IsBootstrapped() {
					log.Debug("could not get root for block", zap.Uint64("height", rootHeight))
					return fmt.Errorf("%w: no results for execHeight", err)
				}
				// If we haven't finished bootstrapping, we can't fail.
				//
				// TODO: we should actually not verify any of this (long p-chain lookbacks)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			break
		}
		if b.Root != root {
			log.Error("block has invalid root", zap.Stringer("blockID", b.ID()), zap.Stringer("expectedRoot", root), zap.Stringer("actualRoot", b.Root))
			panic("root mismatch") // could help us debug
		}
		b.rootHeight = &rootHeight
	} else {
		if b.Root != ids.Empty {
			panic("root should be empty")
		}
	}

	b.vm.Verified(ctx, b)
	log.Info(
		"verified block",
		zap.Stringer("blockID", b.ID()),
		zap.Uint64("height", b.StatefulBlock.Height),
		zap.Any("execHeight", b.execHeight),
		zap.Stringer("parentID", b.Parent()),
		zap.Int("available chunks", len(b.AvailableChunks)),
		zap.Stringer("root", b.Root),
		zap.Int("executed chunks", len(b.ExecutedChunks)),
	)
	success = true
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

	// Mark block as accepted
	b.st = choices.Accepted

	// Prune root (if any)
	if b.rootHeight != nil {
		_, err := b.vm.Engine().PruneRoot(ctx, *b.rootHeight)
		if err != nil {
			return fmt.Errorf("%w: cannot prune root", err)
		}
	}

	// Prune async results (if any)
	var filteredChunks []*FilteredChunk
	if b.execHeight != nil {
		_, fc, err := b.vm.Engine().PruneResults(ctx, *b.execHeight)
		if err != nil {
			return fmt.Errorf("%w: cannot prune results", err)
		}
		filteredChunks = fc
	}

	// Notify the VM that the block has been accepted
	//
	// It is assumed that Accept is invoked atomically such that the slightly
	// misaligned caches (we mark the block state as accepted before we update the
	// repeat protection emaps for chunks/blocks) will not be queried.
	b.vm.Accepted(ctx, b, filteredChunks)

	// Start async execution of chunks (and fetch if missing)
	if b.StatefulBlock.Height > 0 { // nothing to execute in genesis
		b.vm.Engine().Execute(b)
	}

	r := b.vm.Rules(b.StatefulBlock.Timestamp)
	epoch := utils.Epoch(b.StatefulBlock.Timestamp, r.GetEpochDuration())
	b.vm.RecordAcceptedEpoch(epoch)
	b.vm.Logger().Info(
		"accepted block",
		zap.Stringer("blockID", b.ID()),
		zap.Uint64("height", b.StatefulBlock.Height),
		zap.Uint64("epoch", epoch),
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
	b.vm.Accepted(ctx, b, nil)
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
	return fmt.Sprintf("%d:%s root=%s", sb.Height(), sb.ID(), sb.Root)
}
