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
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/utils"
)

const blockHeightRounding = 128

const (
	blockPrefix         = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockIDHeightPrefix = 0x1 // ID -> Height
	blockHeightIDPrefix = 0x2 // Height -> ID (don't always need full block from disk)
	warpSignaturePrefix = 0x3
	warpFetchPrefix     = 0x4
	chunkPrefix         = 0x5 // pruneable chunks (sort by slot)
	filteredChunkPrefix = 0x6 // long-term persistence chunks (TODO: move to flat files or external storage)
)

var (
	isSyncing    = []byte("is_syncing")
	lastAccepted = []byte("last_accepted")

	signatureLRU = &cache.LRU[string, *chain.WarpSignature]{Size: 1024}
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
	expiryHeight := blk.Height() - uint64(vm.config.GetAcceptedBlockWindow())
	var expired bool
	if expiryHeight > 0 && expiryHeight < blk.Height() { // ensure we don't free genesis
		if err := batch.Delete(PrefixBlockKey(expiryHeight)); err != nil {
			return err
		}
		blkID, err := vm.vmDB.Get(PrefixBlockHeightIDKey(expiryHeight))
		if err == nil {
			if err := batch.Delete(PrefixBlockIDHeightKey(ids.ID(blkID))); err != nil {
				return err
			}
		} else {
			vm.Logger().Warn("unable to delete blkID", zap.Uint64("height", expiryHeight), zap.Error(err))
		}
		if err := batch.Delete(PrefixBlockHeightIDKey(expiryHeight)); err != nil {
			return err
		}
		expired = true
		vm.metrics.deletedBlocks.Inc()
		vm.Logger().Info("deleted block", zap.Uint64("height", expiryHeight))
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("%w: unable to update last accepted", err)
	}
	vm.lastAccepted = blk
	vm.acceptedBlocksByID.Put(blk.ID(), blk)
	vm.acceptedBlocksByHeight.Put(blk.Height(), blk.ID())
	if expired && vm.shouldComapct(expiryHeight) {
		go func() {
			start := time.Now()
			if err := vm.CompactDiskBlocks(expiryHeight); err != nil {
				vm.Logger().Error("unable to compact blocks", zap.Error(err))
				return
			}
			vm.Logger().Info("compacted disk blocks", zap.Uint64("end", expiryHeight), zap.Duration("t", time.Since(start)))
		}()
	}
	// TODO: clean up expired chunks
	return nil
}

func BlockDirectory(height uint64) string {
	return fmt.Sprintf("b%s", strconv.FormatUint(utils.RoundUint64(height, blockHeightRounding), 10))
}

func BlockFile(height uint64) string {
	return strconv.FormatUint(height, 10)
}

func (vm *VM) PutDiskBlock(blk *chain.StatelessBlock) error {
	return vm.blobDB.Put(BlockDirectory(blk.Height()), BlockFile(blk.Height()), blk.Bytes())
}

func (vm *VM) GetDiskBlock(ctx context.Context, height uint64) (*chain.StatelessBlock, error) {
	b, err := vm.blobDB.Get(BlockDirectory(height), BlockFile(height))
	if err != nil {
		return nil, err
	}
	return chain.ParseBlock(ctx, b, choices.Accepted, vm)
}

func (vm *VM) HasDiskBlock(height uint64) (bool, error) {
	return vm.blobDB.Has(BlockDirectory(height), BlockFile(height))
}

func (vm *VM) RemoveDiskBlocks(height uint64) ([]uint64, error) {
	names, err := vm.blobDB.Remove(BlockDirectory(height))
	if err != nil {
		return nil, err
	}
	nums := make([]uint64, len(names))
	for i, name := range names {
		num, err := strconv.ParseUint(name, 10, 64)
		if err != nil {
			return nil, err
		}
		nums[i] = num
	}
	return nums, nil
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
	vs, errs := vm.ReadState(context.TODO(), [][]byte{k})
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

// TODO: read directories on restart to determine if should clean
func ChunkDirectory(slot int64) string {
	return fmt.Sprintf("c%s", strconv.FormatInt(slot, 10))
}

func (vm *VM) StoreChunk(chunk *chain.Chunk) error {
	cid, err := chunk.ID()
	if err != nil {
		return err
	}
	b, err := chunk.Marshal()
	if err != nil {
		return err
	}
	return vm.blobDB.Put(ChunkDirectory(chunk.Slot), cid.String(), b)
}

func (vm *VM) GetChunk(slot int64, chunk ids.ID) (*chain.Chunk, error) {
	b, err := vm.blobDB.Get(ChunkDirectory(slot), chunk.String())
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalChunk(b, vm)
}

func (vm *VM) HasChunk(_ context.Context, slot int64, chunk ids.ID) bool {
	// TODO: add error
	has, _ := vm.blobDB.Has(ChunkDirectory(slot), chunk.String())
	return has
}

func (vm *VM) RemoveChunks(slot int64) ([]ids.ID, error) {
	names, err := vm.blobDB.Remove(ChunkDirectory(slot))
	if err != nil {
		return nil, err
	}
	return parseIDs(names)
}

func FilteredChunkDirectory(slot int64) string {
	return fmt.Sprintf("fc%s", strconv.FormatInt(slot, 10))
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
	return vm.blobDB.Put(FilteredChunkDirectory(chunk.Slot), cid.String(), b)
}

func (vm *VM) GetFilteredChunk(slot int64, chunk ids.ID) (*chain.FilteredChunk, error) {
	b, err := vm.blobDB.Get(FilteredChunkDirectory(slot), chunk.String())
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalFilteredChunk(b, vm)
}

func (vm *VM) RemoveFilteredChunks(slot int64) ([]ids.ID, error) {
	names, err := vm.blobDB.Remove(FilteredChunkDirectory(slot))
	if err != nil {
		return nil, err
	}
	return parseIDs(names)
}

func parseIDs(strings []string) ([]ids.ID, error) {
	idsArr := make([]ids.ID, len(strings))
	for i, id := range strings {
		pid, err := ids.FromString(id)
		if err != nil {
			return nil, err
		}
		idsArr[i] = pid
	}
	return idsArr, nil
}
