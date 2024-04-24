// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
)

// compactionOffset is used to randomize the height that we compact
// deleted blocks. This prevents all nodes on the network from deleting
// data from disk at the same time.
var compactionOffset int = -1

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	blockPrefix         = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockIDHeightPrefix = 0x1 // ID -> Height
	blockHeightIDPrefix = 0x2 // Height -> ID (don't always need full block from disk)
)

var (
	isSyncing    = []byte("is_syncing")
	lastAccepted = []byte("last_accepted")
)

func PrefixBlockKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

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

func (vm *VM) shouldComapct(expiryHeight uint64) bool {
	if compactionOffset == -1 {
		compactionOffset = rand.Intn(vm.config.GetBlockCompactionFrequency()) //nolint:gosec
		vm.Logger().Info("setting compaction offset", zap.Int("n", compactionOffset))
	}
	return expiryHeight%uint64(vm.config.GetBlockCompactionFrequency()) == uint64(compactionOffset)
}

// UpdateLastAccepted updates the [lastAccepted] index, stores [blk] on-disk,
// adds [blk] to the [acceptedCache], and deletes any expired blocks from
// disk.
//
// Blocks written to disk are only used when restarting the node. During normal
// operation, we only fetch blocks from memory.
//
// We store blocks by height because it doesn't cause nearly as much
// compaction as storing blocks randomly on-disk (when using [block.ID]).
func (vm *VM) UpdateLastAccepted(blk *chain.StatelessBlock) error {
	batch := vm.vmDB.NewBatch()
	bigEndianHeight := binary.BigEndian.AppendUint64(nil, blk.Height())
	if err := batch.Put(lastAccepted, bigEndianHeight); err != nil {
		return err
	}
	if err := batch.Put(PrefixBlockKey(blk.Height()), blk.Bytes()); err != nil {
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
	return nil
}

func (vm *VM) GetDiskBlock(ctx context.Context, height uint64) (*chain.StatelessBlock, error) {
	b, err := vm.vmDB.Get(PrefixBlockKey(height))
	if err != nil {
		return nil, err
	}
	return chain.ParseBlock(ctx, b, choices.Accepted, vm)
}

func (vm *VM) HasDiskBlock(height uint64) (bool, error) {
	return vm.vmDB.Has(PrefixBlockKey(height))
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

// CompactDiskBlocks forces compaction on the entire range of blocks up to [lastExpired].
//
// This can be used to ensure we clean up all large tombstoned keys on a regular basis instead
// of waiting for the database to run a compaction (and potentially delete GBs of data at once).
func (vm *VM) CompactDiskBlocks(lastExpired uint64) error {
	return vm.vmDB.Compact([]byte{blockPrefix}, PrefixBlockKey(lastExpired))
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
