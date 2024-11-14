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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	evmPrefix = 0xdd

	StorageChunks = 1
	AccountChunks = 2
	CodeChunks    = 1024 * 24 / 64 // the max contract size is 24KB
)

func AccountKey(addr []byte) []byte {
	k := make([]byte, 0, 1+len(addr)+consts.Uint16Len)
	k = append(k, evmPrefix)
	k = append(k, addr...)
	k = binary.BigEndian.AppendUint16(k, AccountChunks)
	return k
}

func StorageKey(addr []byte, key []byte) []byte {
	keyHash := crypto.Keccak256Hash(key)
	k := make([]byte, 0, 1+2*len(addr)+len(key)+consts.Uint16Len)
	k = append(k, evmPrefix)
	k = append(k, addr...)
	k = append(k, keyHash.Bytes()...)
	k = binary.BigEndian.AppendUint16(k, StorageChunks)
	return k
}

func GetStorage(
	ctx context.Context,
	im state.Immutable,
	addr []byte,
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
	addr []byte,
	key, value []byte,
) error {
	k := StorageKey(addr, key)
	return mu.Insert(ctx, k, value)
}

func DeleteStorage(
	ctx context.Context,
	mu state.Mutable,
	addr []byte,
	key []byte,
) error {
	k := StorageKey(addr, key)
	return mu.Remove(ctx, k)
}

func GetAccount(
	ctx context.Context,
	im state.Immutable,
	addr []byte,
) ([]byte, error) {
	k := AccountKey(addr)
	val, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func SetAccount(
	ctx context.Context,
	mu state.Mutable,
	addr []byte,
	account []byte,
) error {
	k := AccountKey(addr)
	return mu.Insert(ctx, k, account)
}

func DeleteAccount(
	ctx context.Context,
	mu state.Mutable,
	addr []byte,
) error {
	k := AccountKey(addr)
	return mu.Remove(ctx, k)
}

func CodeKey(addr []byte) []byte {
	addrHash := crypto.Keccak256Hash(addr)
	k := make([]byte, 0, 1+common.HashLength+consts.Uint16Len)
	k = append(k, evmPrefix)
	k = append(k, addrHash.Bytes()...)
	k = binary.BigEndian.AppendUint16(k, CodeChunks)
	return k
}

func GetCode(
	ctx context.Context,
	im state.Immutable,
	addr common.Address,
) ([]byte, error) {
	k := CodeKey(addr.Bytes())
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
	addr common.Address,
	code []byte,
) error {
	k := CodeKey(addr.Bytes())
	return mu.Insert(ctx, k, code)
}

func GetNonce(
	ctx context.Context,
	im state.Immutable,
	addr []byte,
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
