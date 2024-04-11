// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/keys"
)

const (
	blockIDHeightPrefix = 0x0 // ID -> Height
	blockHeightIDPrefix = 0x1 // Height -> ID (don't always need full block from disk)
	warpSignaturePrefix = 0x2
	warpFetchPrefix     = 0x3
)

var (
	isSyncing    = []byte("is_syncing")
	lastAccepted = []byte("last_accepted")

	signatureLRU = &cache.LRU[string, *chain.WarpSignature]{Size: 1024}

	// chunkLRU is a cache on top of the fileDB byte cache that prevents repeated unmarshaling of chunks
	chunkLRU = cache.NewSizedLRU[ids.ID, *chain.Chunk](2048*units.MiB, func(K ids.ID, V *chain.Chunk) int { return ids.IDLen + V.Size() })
)

func PrefixBlockIDHeightKey(id ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = blockIDHeightPrefix
	copy(k[1:], id[:])
	return k
}

func PrefixBlockHeightIDKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockHeightIDPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func (vm *VM) HasGenesis() (bool, error) {
	return vm.HasDiskBlock(0)
}

func (vm *VM) GetGenesis(ctx context.Context) (*chain.StatelessBlock, error) {
	return vm.GetDiskBlock(ctx, 0)
}

func (vm *VM) SetLastAcceptedHeight(height uint64) error {
	return vm.vmDB.Put(lastAccepted, binary.BigEndian.AppendUint64(nil, height))
}

func (vm *VM) HasLastAccepted() (bool, error) {
	return vm.vmDB.Has(lastAccepted)
}

func (vm *VM) GetLastAcceptedHeight() (uint64, error) {
	b, err := vm.vmDB.Get(lastAccepted)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

// UpdateLastAccepted updates the [lastAccepted] index, stores [blk] on-disk,
// adds [blk] to the [acceptedCache], and deletes any expired blocks from
// disk.
//
// Blocks written to disk are only used when restarting the node. During normal
// operation, we only fetch blocks from memory.
func (vm *VM) UpdateLastAccepted(blk *chain.StatelessBlock) error {
	if err := vm.PutDiskBlock(blk); err != nil {
		return fmt.Errorf("%w: unable to store block", err)
	}
	batch := vm.vmDB.NewBatch()
	bigEndianHeight := binary.BigEndian.AppendUint64(nil, blk.Height())
	if err := batch.Put(lastAccepted, bigEndianHeight); err != nil {
		return err
	}
	if err := batch.Put(PrefixBlockIDHeightKey(blk.ID()), bigEndianHeight); err != nil {
		return err
	}
	blkID := blk.ID()
	if err := batch.Put(PrefixBlockHeightIDKey(blk.Height()), blkID[:]); err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("%w: unable to update last accepted", err)
	}
	vm.lastAccepted = blk
	vm.acceptedBlocksByID.Put(blk.ID(), blk)
	vm.acceptedBlocksByHeight.Put(blk.Height(), blk.ID())
	return nil
}

func (vm *VM) PruneBlockAndChunks(height uint64) error {
	start := time.Now()
	abw := vm.config.GetAcceptedBlockWindow()
	if height < uint64(abw) {
		vm.Logger().Info("not pruning any blocks", zap.Int("minHeight", abw+1))
		return nil
	}
	expiryHeight := height - uint64(abw)
	blk, err := vm.GetDiskBlock(context.TODO(), expiryHeight)
	if errors.Is(err, database.ErrNotFound) {
		vm.Logger().Info("not pruning missing block", zap.Uint64("height", expiryHeight))
		return nil
	}
	batch := vm.vmDB.NewBatch()
	if err := batch.Delete(PrefixBlockIDHeightKey(blk.ID())); err != nil {
		return err
	}
	if err := batch.Delete(PrefixBlockHeightIDKey(expiryHeight)); err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("%w: unable to update last accepted", err)
	}
	// Because fileDB operates directly over an SSD, we can take advantage of the ability
	// to perform concurrent file operations (which are not possible in PebbleDB).
	g, _ := errgroup.WithContext(context.TODO())
	g.Go(func() error {
		if err := vm.RemoveDiskBlock(expiryHeight); err != nil {
			return fmt.Errorf("%w: cannot remove block", err)
		}
		return nil
	})
	for _, cert := range blk.AvailableChunks {
		vm.cm.RemoveStored(cert.Chunk) // ensures we don't delete the same chunk twice
		tcert := cert
		g.Go(func() error {
			if err := vm.RemoveChunk(tcert.Slot, tcert.Chunk); err != nil {
				return fmt.Errorf("%w: cannot remove included chunk", err)
			}
			return nil
		})
	}
	for _, chunk := range blk.ExecutedChunks {
		tchunk := chunk
		g.Go(func() error {
			if err := vm.RemoveFilteredChunk(tchunk); err != nil {
				return fmt.Errorf("%w: cannot remove filtered chunk", err)
			}
			return nil
		})
	}
	uselessChunks := vm.cm.SetStoredMin(blk.StatefulBlock.Timestamp)
	for _, chunk := range uselessChunks {
		tchunk := chunk
		g.Go(func() error {
			if err := vm.RemoveChunk(tchunk.slot, tchunk.chunk); err != nil {
				return fmt.Errorf("%w: cannot remove useless chunk", err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	vm.metrics.deletedBlocks.Inc()
	vm.metrics.deletedUselessChunks.Add(float64(len(uselessChunks)))
	vm.metrics.deletedIncludedChunks.Add(float64(len(blk.AvailableChunks)))
	vm.metrics.deletedFilteredChunks.Add(float64(len(blk.ExecutedChunks)))
	vm.Logger().Info(
		"deleted block",
		zap.Uint64("height", expiryHeight),
		zap.Int("uselessChunks", len(uselessChunks)),
		zap.Int("availableChunks", len(blk.AvailableChunks)),
		zap.Int("executedChunks", len(blk.ExecutedChunks)),
		zap.Duration("t", time.Since(start)),
	)
	return nil
}

func BlockFile(height uint64) string {
	return fmt.Sprintf("b-%s", strconv.FormatUint(height, 10))
}

func (vm *VM) PutDiskBlock(blk *chain.StatelessBlock) error {
	return vm.blobDB.Put(BlockFile(blk.Height()), blk.Bytes(), true)
}

func (vm *VM) GetDiskBlock(ctx context.Context, height uint64) (*chain.StatelessBlock, error) {
	b, err := vm.blobDB.Get(BlockFile(height), true)
	if err != nil {
		return nil, err
	}
	return chain.ParseBlock(ctx, b, choices.Accepted, vm)
}

func (vm *VM) HasDiskBlock(height uint64) (bool, error) {
	return vm.blobDB.Has(BlockFile(height))
}

func (vm *VM) RemoveDiskBlock(height uint64) error {
	return vm.blobDB.Remove(BlockFile(height))
}

func (vm *VM) GetBlockHeightID(height uint64) (ids.ID, error) {
	b, err := vm.vmDB.Get(PrefixBlockHeightIDKey(height))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(b), nil
}

func (vm *VM) GetBlockIDHeight(blkID ids.ID) (uint64, error) {
	b, err := vm.vmDB.Get(PrefixBlockIDHeightKey(blkID))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

func (vm *VM) GetDiskIsSyncing() (bool, error) {
	v, err := vm.vmDB.Get(isSyncing)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v[0] == 0x1, nil
}

func (vm *VM) PutDiskIsSyncing(v bool) error {
	if v {
		return vm.vmDB.Put(isSyncing, []byte{0x1})
	}
	return vm.vmDB.Put(isSyncing, []byte{0x0})
}

func (vm *VM) GetOutgoingWarpMessage(txID ids.ID) (*warp.UnsignedMessage, error) {
	p := vm.c.StateManager().OutgoingWarpKeyPrefix(txID)
	k := keys.EncodeChunks(p, chain.MaxOutgoingWarpChunks)
	vs, errs := vm.ReadState(context.TODO(), []string{k})
	v, err := vs[0], errs[0]
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return warp.ParseUnsignedMessage(v)
}

func PrefixWarpSignatureKey(txID ids.ID, signer *bls.PublicKey) []byte {
	k := make([]byte, 1+consts.IDLen+bls.PublicKeyLen)
	k[0] = warpSignaturePrefix
	copy(k[1:], txID[:])
	copy(k[1+consts.IDLen:], bls.PublicKeyToCompressedBytes(signer))
	return k
}

func (vm *VM) StoreWarpSignature(txID ids.ID, signer *bls.PublicKey, signature []byte) error {
	k := PrefixWarpSignatureKey(txID, signer)
	// Cache any signature we produce for later queries from peers
	if bytes.Equal(vm.pkBytes, bls.PublicKeyToCompressedBytes(signer)) {
		signatureLRU.Put(string(k), chain.NewWarpSignature(vm.pkBytes, signature))
	}
	return vm.vmDB.Put(k, signature)
}

func (vm *VM) GetWarpSignature(txID ids.ID, signer *bls.PublicKey) (*chain.WarpSignature, error) {
	k := PrefixWarpSignatureKey(txID, signer)
	if ws, ok := signatureLRU.Get(string(k)); ok {
		return ws, nil
	}
	v, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ws := &chain.WarpSignature{
		PublicKey: bls.PublicKeyToCompressedBytes(signer),
		Signature: v,
	}
	return ws, nil
}

func (vm *VM) GetWarpSignatures(txID ids.ID) ([]*chain.WarpSignature, error) {
	prefix := make([]byte, 1+consts.IDLen)
	prefix[0] = warpSignaturePrefix
	copy(prefix[1:], txID[:])
	iter := vm.vmDB.NewIteratorWithPrefix(prefix)
	defer iter.Release()

	// Collect all signatures we have for a txID
	signatures := []*chain.WarpSignature{}
	for iter.Next() {
		k := iter.Key()
		signatures = append(signatures, &chain.WarpSignature{
			PublicKey: k[len(k)-bls.PublicKeyLen:],
			Signature: iter.Value(),
		})
	}
	return signatures, iter.Error()
}

func PrefixWarpFetchKey(txID ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = warpFetchPrefix
	copy(k[1:], txID[:])
	return k
}

func (vm *VM) StoreWarpFetch(txID ids.ID) error {
	k := PrefixWarpFetchKey(txID)
	return vm.vmDB.Put(k, binary.BigEndian.AppendUint64(nil, uint64(time.Now().UnixMilli())))
}

func (vm *VM) GetWarpFetch(txID ids.ID) (int64, error) {
	k := PrefixWarpFetchKey(txID)
	v, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	return int64(binary.BigEndian.Uint64(v)), nil
}

// TODO: read blobDB on restart to determine what should clean
func ChunkFile(slot int64, id ids.ID) string {
	return fmt.Sprintf("c-%s-%s", strconv.FormatInt(slot, 10), id)
}

func (vm *VM) StoreChunk(chunk *chain.Chunk) error {
	chunkLRU.Put(chunk.ID(), chunk)
	b, err := chunk.Marshal()
	if err != nil {
		return err
	}
	return vm.blobDB.Put(ChunkFile(chunk.Slot, chunk.ID()), b, false)
}

func (vm *VM) GetChunk(slot int64, chunk ids.ID) (*chain.Chunk, error) {
	if c, ok := chunkLRU.Get(chunk); ok {
		return c, nil
	}
	b, err := vm.blobDB.Get(ChunkFile(slot, chunk), false)
	if errors.Is(err, database.ErrNotFound) {
		// TODO: remove this pattern
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalChunk(b, vm)
}

func (vm *VM) GetChunkBytes(slot int64, chunk ids.ID) ([]byte, error) {
	return vm.blobDB.Get(ChunkFile(slot, chunk), false)
}

func (vm *VM) HasChunk(_ context.Context, slot int64, chunk ids.ID) bool {
	if _, ok := chunkLRU.Get(chunk); ok {
		return true
	}
	// TODO: add error
	has, _ := vm.blobDB.Has(ChunkFile(slot, chunk))
	return has
}

func (vm *VM) RemoveChunk(slot int64, chunk ids.ID) error {
	chunkLRU.Evict(chunk)
	return vm.blobDB.Remove(ChunkFile(slot, chunk))
}

func FilteredChunkFile(chunk ids.ID) string {
	return fmt.Sprintf("fc-%s", chunk)
}

func (vm *VM) StoreFilteredChunk(chunk *chain.FilteredChunk) error {
	cid, err := chunk.ID()
	if err != nil {
		return err
	}
	b, err := chunk.Marshal()
	if err != nil {
		return err
	}
	return vm.blobDB.Put(FilteredChunkFile(cid), b, true)
}

func (vm *VM) GetFilteredChunk(chunk ids.ID) (*chain.FilteredChunk, error) {
	b, err := vm.blobDB.Get(FilteredChunkFile(chunk), true)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalFilteredChunk(b, vm)
}

func (vm *VM) GetFilteredChunkBytes(chunk ids.ID) ([]byte, error) {
	return vm.blobDB.Get(FilteredChunkFile(chunk), true)
}

func (vm *VM) RemoveFilteredChunk(chunk ids.ID) error {
	return vm.blobDB.Remove(FilteredChunkFile(chunk))
}
