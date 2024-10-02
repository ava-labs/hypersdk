// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/layout"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	stateLayout layout.Layout,
	im state.Immutable,
	addr codec.Address,
) (uint64, error) {
	_, bal, _, err := getBalance(ctx, stateLayout, im, addr)
	return bal, err
}

func getBalance(
	ctx context.Context,
	stateLayout layout.Layout,
	im state.Immutable,
	addr codec.Address,
) ([]byte, uint64, bool, error) {
	k := stateLayout.NewBalanceKey(addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

// Used to serve RPC queries
func GetBalanceFromState(
	ctx context.Context,
	stateLayout layout.Layout,
	f ReadState,
	addr codec.Address,
) (uint64, error) {
	k := stateLayout.NewBalanceKey(addr)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}

func innerGetBalance(
	v []byte,
	err error,
) (uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	val, err := database.ParseUInt64(v)
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}

func SetBalance(
	ctx context.Context,
	stateLayout layout.Layout,
	mu state.Mutable,
	addr codec.Address,
	balance uint64,
) error {
	k := stateLayout.NewBalanceKey(addr)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	balance uint64,
) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(
	ctx context.Context,
	stateLayout layout.Layout,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
	create bool,
) (uint64, error) {
	key, bal, exists, err := getBalance(ctx, stateLayout, mu, addr)
	if err != nil {
		return 0, err
	}
	// Don't add balance if account doesn't exist. This
	// can be useful when processing fee refunds.
	if !exists && !create {
		return 0, nil
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
	return nbal, setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	stateLayout layout.Layout,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) (uint64, error) {
	key, bal, ok, err := getBalance(ctx, stateLayout, mu, addr)
	if !ok {
		return 0, ErrInvalidAddress
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
