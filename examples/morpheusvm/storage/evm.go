// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

const (
	accountPrefix byte = metadata.DefaultMinimumPrefix + iota
	storagePrefix
	codePrefix

	StorageChunks = 1
	AccountChunks = 2
	CodeChunks    = 1024 * 24 / 64 // the max contract size is 24KB
)

func AccountKey(addr common.Address) []byte {
	k := make([]byte, consts.ByteLen+common.AddressLength+consts.Uint16Len)
	k[0] = accountPrefix
	copy(k[1:1+common.AddressLength], addr.Bytes())
	binary.BigEndian.PutUint16(k[1+common.AddressLength:], AccountChunks)
	return k
}

func StorageKey(addr common.Address, key []byte) []byte {
	k := make([]byte, consts.ByteLen+common.AddressLength+len(key)+consts.Uint16Len)
	k[0] = storagePrefix
	copy(k[1:1+common.AddressLength], addr.Bytes())
	copy(k[1+common.AddressLength:], key)
	binary.BigEndian.PutUint16(k[1+common.AddressLength+len(key):], StorageChunks)
	return k
}

func CodeKey(codeHash common.Hash) []byte {
	k := make([]byte, consts.ByteLen+common.HashLength+consts.Uint16Len)
	k[0] = codePrefix
	copy(k[1:1+common.HashLength], codeHash[:])
	binary.BigEndian.PutUint16(k[1+common.HashLength:], CodeChunks)
	return k
}

func GetStorage(
	ctx context.Context,
	im state.Immutable,
	addr common.Address,
	key []byte,
) ([]byte, error) {
	k := StorageKey(addr, key)
	val, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func SetStorage(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
	key, value []byte,
) error {
	k := StorageKey(addr, key)
	return mu.Insert(ctx, k, value)
}

func DeleteStorage(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
	key []byte,
) error {
	k := StorageKey(addr, key)
	return mu.Remove(ctx, k)
}

func GetAccount(
	ctx context.Context,
	im state.Immutable,
	addr common.Address,
) ([]byte, error) {
	k := AccountKey(addr)
	val, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		newAcc := types.StateAccount{
			Nonce:    0,
			Balance:  uint256.NewInt(0),
			Root:     common.Hash{},
			CodeHash: []byte{},
		}
		encoded, err := EncodeAccount(&newAcc)
		if err != nil {
			return nil, err
		}
		return encoded, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func SetAccount(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
	account []byte,
) error {
	k := AccountKey(addr)
	return mu.Insert(ctx, k, account)
}

func DeleteAccount(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
) error {
	k := AccountKey(addr)
	return mu.Remove(ctx, k)
}

func GetCode(
	ctx context.Context,
	im state.Immutable,
	_ common.Address,
	codeHash common.Hash,
) ([]byte, error) {
	k := CodeKey(codeHash)
	val, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func SetCode(
	ctx context.Context,
	mu state.Mutable,
	_ common.Address,
	codeHash common.Hash,
	code []byte,
) error {
	k := CodeKey(codeHash)
	return mu.Insert(ctx, k, code)
}

func GetNonce(
	ctx context.Context,
	im state.Immutable,
	addr common.Address,
) (uint64, error) {
	k := AccountKey(addr)
	val, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	account, err := DecodeAccount(val)
	if err != nil {
		return 0, err
	}
	return account.Nonce, nil
}
