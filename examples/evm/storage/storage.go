// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

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
	// if the account does not exist, we need to create it according to the EVM spec
	if err == database.ErrNotFound {
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
