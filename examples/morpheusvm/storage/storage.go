// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// State
// 0x0/ (hypersdk-height)
// 0x1/ (hypersdk-timestamp)
// 0x2/ (hypersdk-fee)
//
// 0x3/ (balance)
//   -> [owner] => balance

const balancePrefix byte = metadata.DefaultMinimumPrefix

const BalanceChunks uint16 = 1

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	addr []byte,
) (uint64, error) {
	_, bal, _, err := getBalance(ctx, im, addr[:])
	return bal, err
}

func getBalance(
	ctx context.Context,
	im state.Immutable,
	addr []byte,
) ([]byte, uint64, bool, error) {
	k := AccountKey(addr)
	val, err := im.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	account, err := DecodeAccount(val)
	if err != nil {
		return nil, 0, false, fmt.Errorf("failed to decode account: %w", err)
	}
	return k, account.Balance.Uint64(), true, nil
}

func SetBalance(
	ctx context.Context,
	mu state.Mutable,
	addr []byte,
	balance uint64,
) error {
	k := AccountKey(addr)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	balance uint64,
) error {
	account, err := GetAccount(ctx, mu, key)
	if err != nil {
		return err
	}
	accountDecoded, err := DecodeAccount(account)
	if err != nil {
		return err
	}
	accountDecoded.Balance = uint256.NewInt(0).SetUint64(balance)
	encoded, err := EncodeAccount(accountDecoded)
	if err != nil {
		return err
	}
	return mu.Insert(ctx, key, encoded)
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	addr []byte,
	amount uint64,
) (uint64, error) {
	key, bal, _, err := getBalance(ctx, mu, addr)
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	if nbal > consts.MaxUint64 {
		return 0, fmt.Errorf(
			"%w: balance overflow (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	addr []byte,
	amount uint64,
) (uint64, error) {
	key, bal, ok, err := getBalance(ctx, mu, addr)
	if !ok {
		return 0, ErrInvalidBalance
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
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
	p := codec.NewReader(data, len(data))
	var account types.StateAccount
	account.Nonce = p.UnpackUint64(false)
	account.Balance = uint256.NewInt(0).SetUint64(p.UnpackUint64(false))
	rt := make([]byte, 32)
	p.UnpackFixedBytes(len(rt), &rt)
	account.Root = common.BytesToHash(rt)
	p.UnpackBytes(-1, false, &account.CodeHash)
	return &account, p.Err()
}
