// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

func FromEVMAddress(addr common.Address) codec.Address {
	var address codec.Address
	copy(address[13:], addr[:])
	return address
}

func ToEVMAddress(addr codec.Address) common.Address {
	return common.Address(addr[13:])
}

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

func CodeKey(addr common.Address) []byte {
	k := make([]byte, consts.ByteLen+common.AddressLength+consts.Uint16Len)
	k[0] = codePrefix
	copy(k[1:1+common.AddressLength], addr.Bytes())
	binary.BigEndian.PutUint16(k[1+common.AddressLength:], CodeChunks)
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

// Is this correct or do we need to grab the balance from a separate key and load that into the account as well?
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
	addr common.Address,
) ([]byte, error) {
	k := CodeKey(addr)
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
	k := CodeKey(addr)
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

func EncodeAccount(account *types.StateAccount) ([]byte, error) {
	p := codec.NewWriter(0, consts.MaxInt)
	p.PackUint64(account.Nonce)
	p.PackUint64(account.Balance.Uint64())
	p.PackFixedBytes(account.Root.Bytes())
	p.PackBytes(account.CodeHash)
	return p.Bytes(), p.Err()
}

func DecodeAccount(data []byte) (*types.StateAccount, error) {
	var account types.StateAccount

	if len(data) == 0 { // todo: check if this is correct
		account = *types.NewEmptyStateAccount()
		return &account, nil
	}
	p := codec.NewReader(data, len(data))
	account.Nonce = p.UnpackUint64(false)
	account.Balance = uint256.NewInt(0).SetUint64(p.UnpackUint64(false))
	rt := make([]byte, 32)
	p.UnpackFixedBytes(len(rt), &rt)
	account.Root = common.BytesToHash(rt)
	p.UnpackBytes(-1, false, &account.CodeHash)
	return &account, p.Err()
}
