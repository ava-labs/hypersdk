// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/keys"
)

// compactionOffset is used to randomize the height that we compact
// deleted blocks. This prevents all nodes on the network from deleting
// data from disk at the same time.
var compactionOffset int = -1

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	blockPrefix         = 0x0
	warpSignaturePrefix = 0x1
	warpFetchPrefix     = 0x2
)

var (
	isSyncing    = []byte("is_syncing")
	lastAccepted = []byte("last_accepted")

	signatureLRU = &cache.LRU[string, *chain.WarpSignature]{Size: 1024}
)

func PrefixBlockHeightKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func (vm *VM) HasGenesis() (bool, error) {
	return vm.HasDiskBlock(0)
}

func (vm *VM) GetGenesis() (*chain.StatefulBlock, error) {
	return vm.GetDiskBlock(0)
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
	if err := batch.Put(lastAccepted, binary.BigEndian.AppendUint64(nil, blk.Height())); err != nil {
		return err
	}
	if err := batch.Put(PrefixBlockHeightKey(blk.Height()), blk.Bytes()); err != nil {
		return err
	}
	expiryHeight := blk.Height() - uint64(vm.config.GetAcceptedBlockWindow())
	var expired bool
	if expiryHeight > 0 && expiryHeight < blk.Height() { // ensure we don't free genesis
		if err := batch.Delete(PrefixBlockHeightKey(expiryHeight)); err != nil {
			return err
		}
		expired = true
		vm.metrics.deletedBlocks.Inc()
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

func (vm *VM) GetDiskBlock(height uint64) (*chain.StatefulBlock, error) {
	b, err := vm.vmDB.Get(PrefixBlockHeightKey(height))
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalBlock(b, vm)
}

func (vm *VM) HasDiskBlock(height uint64) (bool, error) {
	return vm.vmDB.Has(PrefixBlockHeightKey(height))
}

// CompactDiskBlocks forces compaction on the entire range of blocks up to [lastExpired].
//
// This can be used to ensure we clean up all large tombstoned keys on a regular basis instead
// of waiting for the database to run a compaction (and potentially delete GBs of data at once).
func (vm *VM) CompactDiskBlocks(lastExpired uint64) error {
	return vm.vmDB.Compact([]byte{blockPrefix}, PrefixBlockHeightKey(lastExpired))
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
	copy(k[1+consts.IDLen:], bls.PublicKeyToBytes(signer))
	return k
}

func (vm *VM) StoreWarpSignature(txID ids.ID, signer *bls.PublicKey, signature []byte) error {
	k := PrefixWarpSignatureKey(txID, signer)
	// Cache any signature we produce for later queries from peers
	if bytes.Equal(vm.pkBytes, bls.PublicKeyToBytes(signer)) {
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
		PublicKey: bls.PublicKeyToBytes(signer),
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
