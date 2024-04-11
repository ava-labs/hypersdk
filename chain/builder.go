// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

const (
	defaultChunks = 16
)

// TODO: move to block file?
func BuildBlock(
	ctx context.Context,
	vm VM,
	pHeight uint64,
	parent *StatelessBlock,
) (*StatelessBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	// Select next timestamp
	next := time.Now()
	nextTime := next.UnixMilli()
	r := vm.Rules(nextTime)
	if nextTime < parent.StatefulBlock.Timestamp+r.GetMinBlockGap() {
		return nil, ErrTimestampTooEarly
	}
	b := NewBlock(vm, pHeight, parent, nextTime)

	// Attempt to add valid certs that are not expired (1 per validator)
	//
	// TODO: consider allowing up to N per validator
	b.chunks = set.NewSet[ids.ID](16)
	b.AvailableChunks = make([]*ChunkCertificate, 0, 16)
	if pHeight > 0 { // even if epoch is set, we use this height to verify warp messages
		vm.StartCertStream(ctx)
		restorableChunks := []*ChunkCertificate{}
		for {
			// TODO: ensure chunk producer is in this epoch
			// TODO: ensure only 1 chunk per producer
			// TOOD: prefer old chunks to new chunks for a given producer

			// It is assumed that [NextChunkCertificate] will never return a duplicate chunk.
			cert, ok := vm.StreamCert(ctx)
			if !ok {
				break
			}

			// Check that we actually have the chunk
			if !vm.HasChunk(ctx, cert.Slot, cert.Chunk) {
				log.Debug("skipping certificate of chunk we don't have", zap.Stringer("chunkID", cert.Chunk))
				restorableChunks = append(restorableChunks, cert)
				continue
			}

			// Check that certificate can be in block
			if cert.Slot < b.StatefulBlock.Timestamp {
				log.Debug("skipping expired chunk", zap.Stringer("chunkID", cert.Chunk))
				restorableChunks = append(restorableChunks, cert) // wait for this to get cleared via "SetMin" (may still want if reorg)
				continue
			}
			if cert.Slot > b.StatefulBlock.Timestamp+r.GetValidityWindow() {
				log.Debug("skipping chunk too far in the future", zap.Stringer("chunkID", cert.Chunk))
				restorableChunks = append(restorableChunks, cert)
				continue
			}

			// Check if the chunk is a repeat
			repeats, err := parent.IsRepeatChunk(ctx, []*ChunkCertificate{cert}, set.NewBits())
			if err != nil {
				log.Warn("failed to check if chunk is a repeat", zap.Stringer("chunkID", cert.Chunk), zap.Error(err))
				b.vm.RecordBlockBuildCertDropped()
				continue
			}
			if repeats.Len() > 0 {
				log.Debug("skipping duplicate chunk", zap.Stringer("chunkID", cert.Chunk))
				b.vm.RecordBlockBuildCertDropped()
				continue
			}

			// Assume chunk is valid and there are P-Chain heights for applicable epochs because it has sufficient signatures
			//
			// TODO: consider validating anyways

			// Add chunk to block
			b.chunks.Add(cert.Chunk)
			b.AvailableChunks = append(b.AvailableChunks, cert)
			b.vm.Logger().Debug(
				"included chunk in block",
				zap.Uint64("block", b.StatefulBlock.Height),
				zap.Stringer("chunkID", cert.Chunk),
				zap.Uint64("epoch", utils.Epoch(cert.Slot, r.GetEpochDuration())),
			)
		}
		vm.FinishCertStream(ctx, restorableChunks)
	}

	// Fetch executed blocks
	depth := r.GetBlockExecutionDepth()
	if b.StatefulBlock.Height > depth {
		execHeight := b.StatefulBlock.Height - depth
		executed, checksum, err := vm.Engine().Results(execHeight)
		if err != nil {
			vm.RestoreChunkCertificates(ctx, b.AvailableChunks)
			return nil, err
		}
		b.execHeight = &execHeight
		b.ExecutedChunks = executed
		b.Checksum = checksum
	}

	// Populate all fields in block
	b.built = true
	bytes, err := b.Marshal()
	if err != nil {
		vm.RestoreChunkCertificates(ctx, b.AvailableChunks)
		return nil, err
	}
	b.id = utils.ToID(bytes)
	b.t = time.UnixMilli(b.StatefulBlock.Timestamp)
	b.bytes = bytes
	b.parent = parent
	epoch := utils.Epoch(nextTime, r.GetEpochDuration())
	log.Info(
		"built block",
		zap.Stringer("blockID", b.ID()),
		zap.Uint64("height", b.StatefulBlock.Height),
		zap.Uint64("pHeight", pHeight),
		zap.Uint64("epoch", epoch),
		zap.Any("execHeight", b.execHeight),
		zap.Stringer("parentID", b.Parent()),
		zap.Int("available chunks", len(b.AvailableChunks)),
		zap.Int("executed chunks", len(b.ExecutedChunks)),
		zap.Stringer("checksum", b.Checksum),
		zap.Int("size", len(b.bytes)),
	)
	return b, nil
}
