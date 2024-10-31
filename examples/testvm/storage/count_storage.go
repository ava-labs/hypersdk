// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

func GetCounter(ctx context.Context, im state.Immutable, address codec.Address) (uint64, error) {
	countBytes, err := im.GetValue(ctx, CounterKey(address))
	return innerGetCounter(countBytes, err)
}

func IncreaseCounter(
	ctx context.Context,
	mu state.Mutable,
	address codec.Address, amount uint64,
) error {
	val, err := GetCounter(ctx, mu, address)
	if err != nil {
		return err
	}

	val += amount

	return SetCounter(ctx, mu, address, val)
}

func SetCounter(ctx context.Context, mu state.Mutable, address codec.Address, count uint64) error {
	countBytes := binary.BigEndian.AppendUint64(nil, count)
	return mu.Insert(ctx, CounterKey(address), countBytes)
}

func GetCounterFromState(
	ctx context.Context,
	f ReadState,
	address codec.Address,
) (uint64, error) {
	k := CounterKey(address)
	values, errs := f(ctx, [][]byte{k})
	return innerGetCounter(values[0], errs[0])
}

func CounterKey(address codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen)
	k = append(k, counterPrefix)
	k = append(k, address[:]...)
	return
}

func innerGetCounter(bytes []byte, err error) (uint64, error) {
	// return default value if not found
	if err == database.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	val, err := database.ParseUInt64(bytes)
	if err != nil {
		return 0, err
	}
	return val, nil
}