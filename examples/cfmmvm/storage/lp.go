// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	hconsts "github.com/ava-labs/hypersdk/consts"
)

type ComparisonValue int

const (
	LessThan ComparisonValue = iota - 1
	Equal
	GreaterThan
)

func LiquidityPoolKey(liquidityPoolAddress codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+hconsts.Uint16Len)
	k[0] = liquidityPoolPrefix
	copy(k[1:1+codec.AddressLen], liquidityPoolAddress[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], LiquidityPoolChunks)
	return k
}

// TODO: rewrite all of this logic
// Ordering of tokenX, tokenY handled by this function during address generation
func LiquidityPoolAddress(tokenX codec.Address, tokenY codec.Address) codec.Address {
	comp := CompareAddress(tokenX, tokenY)
	var firstAddress, secondAddress codec.Address
	switch comp {
	case LessThan:
		firstAddress = tokenX
		secondAddress = tokenY
	default:
		firstAddress = tokenY
		secondAddress = tokenX
	}
	v := make([]byte, codec.AddressLen+codec.AddressLen)
	copy(v, firstAddress[:])
	copy(v[codec.AddressLen:], secondAddress[:])
	id := utils.ToID(v)
	return codec.CreateAddress(consts.LIQUIDITYPOOLID, id)
}

func LiqudityPoolTokenAddress(liquidityPool codec.Address) codec.Address {
	id := utils.ToID(liquidityPool[:])
	return codec.CreateAddress(consts.LIQUIDITYPOOLTOKENID, id)
}

func SetLiquidityPool(
	ctx context.Context,
	mu state.Mutable,
	liquidityPoolAddress codec.Address,
	functionID uint8,
	tokenX codec.Address,
	tokenY codec.Address,
	fee uint64,
	feeTo codec.Address,
	reserveX uint64,
	reserveY uint64,
	liquidityToken codec.Address,
	kLast uint64,
) error {
	k := LiquidityPoolKey(liquidityPoolAddress)
	v := make([]byte, hconsts.Uint64Len+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+hconsts.Uint64Len+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len)
	// Inserting functionID
	v[0] = functionID
	// Inserting tokenX
	copy(v[hconsts.ByteLen:], tokenX[:])
	// Inserting tokenY
	copy(v[hconsts.ByteLen+codec.AddressLen:], tokenY[:])
	// Inserting fee
	binary.BigEndian.PutUint64(v[hconsts.ByteLen+codec.AddressLen+codec.AddressLen:], fee)
	// Inserting feeTo
	copy(v[hconsts.ByteLen+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len:], feeTo[:])
	// Inserting reserveX
	binary.BigEndian.PutUint64(v[hconsts.ByteLen+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen:], reserveX)
	// Inserting reserveY
	binary.BigEndian.PutUint64(v[hconsts.ByteLen+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len:], reserveY)
	// Inserting liquidityToken
	copy(v[hconsts.ByteLen+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len+hconsts.Uint64Len:], liquidityToken[:])
	// Inserting kLast
	binary.BigEndian.PutUint64(v[hconsts.ByteLen+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len+hconsts.Uint64Len+codec.AddressLen:], kLast)
	return mu.Insert(ctx, k, v)
}

func GetLiquidityPoolNoController(
	ctx context.Context,
	mu state.Immutable,
	poolAddress codec.Address,
) (uint8, codec.Address, codec.Address, uint64, codec.Address, uint64, uint64, codec.Address, uint64, error) {
	k := LiquidityPoolKey(poolAddress)
	v, err := mu.GetValue(ctx, k)
	if err != nil {
		return 0, codec.EmptyAddress, codec.EmptyAddress, 0, codec.EmptyAddress, 0, 0, codec.EmptyAddress, 0, err
	}
	return innerGetLiquidityPool(v)
}

func innerGetLiquidityPool(
	v []byte,
) (uint8, codec.Address, codec.Address, uint64, codec.Address, uint64, uint64, codec.Address, uint64, error) {
	functionID := v[0]
	tokenX := codec.Address(v[hconsts.Uint64Len : hconsts.Uint64Len+codec.AddressLen])
	tokenY := codec.Address(v[hconsts.Uint64Len+codec.AddressLen : hconsts.Uint64Len+codec.AddressLen+codec.AddressLen])
	fee := binary.BigEndian.Uint64(v[hconsts.Uint64Len+codec.AddressLen+codec.AddressLen:])
	feeTo := codec.Address(v[hconsts.Uint64Len+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len:])
	reserveX := binary.BigEndian.Uint64(v[hconsts.Uint64Len+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen:])
	reserveY := binary.BigEndian.Uint64(v[hconsts.Uint64Len+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len:])
	lpTokenAddress := codec.Address(v[hconsts.Uint64Len+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len+hconsts.Uint64Len:])
	kLast := binary.BigEndian.Uint64(v[hconsts.Uint64Len+codec.AddressLen+codec.AddressLen+hconsts.Uint64Len+codec.AddressLen+hconsts.Uint64Len+hconsts.Uint64Len+codec.AddressLen:])
	return functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddress, kLast, nil
}

func CompareAddress(a codec.Address, b codec.Address) ComparisonValue {
	for i := range a {
		if a[i] < b[i] {
			return LessThan
		} else if a[i] > b[i] {
			return GreaterThan
		}
	}
	return Equal
}

func LiquidityPoolExists(
	ctx context.Context,
	mu state.Immutable,
	poolAddress codec.Address,
) bool {
	v, err := mu.GetValue(ctx, LiquidityPoolKey(poolAddress))
	return v != nil && err == nil
}
