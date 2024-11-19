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
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

func ConvertAddress(addr codec.Address) common.Address {
	return common.BytesToAddress(addr[:])
}

// State
// 0x0/ (hypersdk-height)
// 0x1/ (hypersdk-timestamp)
// 0x2/ (hypersdk-fee)
//
// 0x3/ (balance)
//   -> [owner] => balance

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	addr common.Address,
) (balance uint64, err error) {
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
		return 0, fmt.Errorf("failed to decode account: %w", err)
	}
	return account.Balance.Uint64(), nil
}

func SetBalance(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
	balance uint64,
) error {
	account, err := GetAccount(ctx, mu, addr)
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
	return SetAccount(ctx, mu, addr, encoded)
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
	amount uint64,
) (uint64, error) {
	bal, err := GetBalance(ctx, mu, addr)
	if err == database.ErrNotFound { // if the account does not exist, we need to create it according to the EVM spec
		encoded, err := EncodeAccount(&types.StateAccount{
			Nonce:    0,
			Balance:  uint256.NewInt(0).SetUint64(amount),
			Root:     common.Hash{},
			CodeHash: []byte{},
		})
		if err != nil {
			return 0, err
		}
		err = mu.Insert(ctx, AccountKey(addr), encoded)
		if err != nil {
			return 0, err
		}
		return amount, nil
	}
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
	return nbal, SetBalance(ctx, mu, addr, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	addr common.Address,
	amount uint64,
) (uint64, error) {
	bal, err := GetBalance(ctx, mu, addr)
	if err == database.ErrNotFound {
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
	return nbal, SetBalance(ctx, mu, addr, nbal)
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
