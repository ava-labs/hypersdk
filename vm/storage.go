// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	idPrefix            = 0x0
	heightPrefix        = 0x1
	warpSignaturePrefix = 0x2
	warpFetchPrefix     = 0x3
)

var (
	lastAccepted = []byte("last_accepted")
	isSyncing    = []byte("is_syncing")

	signatureLRU = &cache.LRU[string, *chain.WarpSignature]{Size: 1024}
)

func PrefixBlockIDKey(id ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = idPrefix
	copy(k[1:], id[:])
	return k
}

func PrefixBlockHeightKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = heightPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func (vm *VM) SetLastAccepted(block *chain.StatelessBlock) error {
	var (
		bid  = block.ID()
		vmDB = vm.vmDB
	)
	if err := vmDB.Put(lastAccepted, bid[:]); err != nil {
		return err
	}
	if err := vmDB.Put(PrefixBlockIDKey(bid), block.Bytes()); err != nil {
		return err
	}
	// TODO: store block bytes at height to reduce amount of compaction
	if err := vmDB.Put(PrefixBlockHeightKey(block.Height()), bid[:]); err != nil {
		return err
	}
	return nil
}

func (vm *VM) HasLastAccepted() (bool, error) {
	return vm.vmDB.Has(lastAccepted)
}

func (vm *VM) GetLastAccepted() (ids.ID, error) {
	v, err := vm.vmDB.Get(lastAccepted)
	if errors.Is(err, database.ErrNotFound) {
		return ids.ID{}, nil
	}
	if err != nil {
		return ids.ID{}, err
	}
	return ids.ToID(v)
}

func (vm *VM) GetDiskBlock(bid ids.ID) (*chain.StatefulBlock, error) {
	b, err := vm.vmDB.Get(PrefixBlockIDKey(bid))
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalBlock(b, vm)
}

func (vm *VM) DeleteDiskBlock(bid ids.ID) error {
	return vm.vmDB.Delete(PrefixBlockIDKey(bid))
}

func (vm *VM) GetDiskBlockIDAtHeight(height uint64) (ids.ID, error) {
	v, err := vm.vmDB.Get(PrefixBlockHeightKey(height))
	if err != nil {
		return ids.Empty, nil
	}
	return ids.ToID(v)
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
	k := vm.c.StateManager().OutgoingWarpKey(txID)
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
	return vm.vmDB.Put(k, binary.BigEndian.AppendUint64(nil, uint64(time.Now().Unix())))
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
