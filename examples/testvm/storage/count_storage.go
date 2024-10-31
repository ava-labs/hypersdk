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
	// return default value if not found
	if err == database.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	val, err := database.ParseUInt64(countBytes)
	if err != nil {
		return 0, err
	}
	return val, nil
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
	return nil
}

func SetCounter(ctx context.Context, mu state.Mutable, address codec.Address, count uint64) error {
	countBytes := binary.BigEndian.AppendUint64(nil, count)
	return mu.Insert(ctx, CounterKey(address), countBytes)
}

func CounterKey(address codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen)
	k = append(k, counterPrefix)
	k = append(k, address[:]...)
	return
}
