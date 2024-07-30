// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Contains read/write logic pertaining to the NFT marketplace
package storage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

func MarketplaceOrderStateKey(
	orderID ids.ID,
) []byte {
	k := make([]byte, consts.Uint16Len+ids.IDLen+consts.Uint16Len)
	k[0] = marketplaceOrderPrefix
	copy(k[1:], orderID[:])
	binary.BigEndian.PutUint16(k[1+ids.IDLen:], MarketplaceOrderStateChunks)
	return k
}

func GenerateOrderID(
	parentCollection codec.Address,
	instanceNum uint32,
	price uint64,
) (ids.ID, error) {
	v := make([]byte, codec.AddressLen+consts.Uint32Len+consts.Uint64Len)
	copy(v, parentCollection[:])
	binary.BigEndian.PutUint32(v[codec.AddressLen:], instanceNum)
	binary.BigEndian.PutUint64(v[codec.AddressLen+consts.Uint32Len:], price)
	id := hashing.ComputeHash256Array(v)
	return id, nil
}

func CreateMarketplaceOrder(
	ctx context.Context,
	mu state.Mutable,
	parentCollection codec.Address,
	instanceNum uint32,
	price uint64,
) (ids.ID, error) {
	// Generate orderID + state key
	orderID, err := GenerateOrderID(parentCollection, instanceNum, price)
	if err != nil {
		return ids.Empty, err
	}
	orderStateKey := MarketplaceOrderStateKey(orderID)
	// Check that order does not already exist
	if _, err := mu.GetValue(ctx, orderStateKey); err == nil {
		return ids.Empty, ErrOrderAlreadyExists
	}

	v := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(v, price)

	// Order is new, complete execution
	return orderID, mu.Insert(ctx, orderStateKey, v)
}

func GetMarketplaceOrder(
	ctx context.Context,
	f ReadState,
	orderID ids.ID,
) (uint64, error) {
	k := MarketplaceOrderStateKey(orderID)
	values, errs := f(ctx, [][]byte{k})
	if errs[0] != nil {
		return 0, errs[0]
	}
	return binary.BigEndian.Uint64(values[0]), nil
}

func GetMarketplaceOrderNoController(
	ctx context.Context,
	mu state.Mutable,
	orderID ids.ID,
) (uint64, error) {
	k := MarketplaceOrderStateKey(orderID)
	v, err := mu.GetValue(ctx, k)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(v), nil
}
