// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	idPrefix     = 0x0
	heightPrefix = 0x1
)

var (
	lastAccepted = []byte("last_accepted")
	isSyncing    = []byte("is_syncing")
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
		bid     = block.ID()
		blockDB = vm.blockDB
	)
	if err := blockDB.Put(lastAccepted, bid[:]); err != nil {
		return err
	}
	if err := blockDB.Put(PrefixBlockIDKey(bid), block.Bytes()); err != nil {
		return err
	}
	// TODO: store block bytes at height to reduce amount of compaction
	if err := blockDB.Put(PrefixBlockHeightKey(block.Height()), bid[:]); err != nil {
		return err
	}
	return nil
}

func (vm *VM) HasLastAccepted() (bool, error) {
	return vm.blockDB.Has(lastAccepted)
}

func (vm *VM) GetLastAccepted() (ids.ID, error) {
	v, err := vm.blockDB.Get(lastAccepted)
	if errors.Is(err, database.ErrNotFound) {
		return ids.ID{}, nil
	}
	if err != nil {
		return ids.ID{}, err
	}
	return ids.ToID(v)
}

func (vm *VM) GetDiskBlock(bid ids.ID) (*chain.StatefulBlock, error) {
	b, err := vm.blockDB.Get(PrefixBlockIDKey(bid))
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalBlock(b, vm)
}

func (vm *VM) DeleteDiskBlock(bid ids.ID) error {
	return vm.blockDB.Delete(PrefixBlockIDKey(bid))
}

func (vm *VM) GetDiskBlockIDAtHeight(height uint64) (ids.ID, error) {
	v, err := vm.blockDB.Get(PrefixBlockHeightKey(height))
	if err != nil {
		return ids.Empty, nil
	}
	return ids.ToID(v)
}

func (vm *VM) GetDiskIsSyncing() (bool, error) {
	v, err := vm.blockDB.Get(isSyncing)
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
		return vm.blockDB.Put(isSyncing, []byte{0x1})
	}
	return vm.blockDB.Put(isSyncing, []byte{0x0})
}
