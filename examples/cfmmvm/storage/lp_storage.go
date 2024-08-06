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

	lconsts "github.com/ava-labs/hypersdk/consts"
)

type ComparisonValue int

const (
	LessThan ComparisonValue = iota - 1
	Equal
	GreaterThan
)

func LiquidityPoolKey(liquidityPoolAddress codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+lconsts.Uint16Len)
	k[0] = liquidityPoolPrefix
	copy(k[1:1+codec.AddressLen], liquidityPoolAddress[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], LiquidityPoolChunks)
	return k
}

// Ordering of tokenX, tokenY handled by this function during address generation
func LiquidityPoolAddress(tokenX codec.Address, tokenY codec.Address) (codec.Address, error) {
	comp := CompareAddress(tokenX, tokenY)
	var firstAddress, secondAddress codec.Address
	switch comp {
	case LessThan:
		firstAddress = tokenX
		secondAddress = tokenY
	case GreaterThan:
		firstAddress = tokenY
		secondAddress = tokenX
	default:
		return codec.EmptyAddress, ErrIdenticalAddresses
	}
	v := make([]byte, codec.AddressLen+codec.AddressLen)
	copy(v, firstAddress[:])
	copy(v[codec.AddressLen:], secondAddress[:])
	id := utils.ToID(v)
	return codec.CreateAddress(consts.LIQUIDITYPOOLID, id), nil
}

func LiqudityPoolTokenAddress(liquidityPool codec.Address) codec.Address {
	id := utils.ToID(liquidityPool[:])
	return codec.CreateAddress(consts.LIQUIDITYPOOLTOKENID, id)
}

func SetLiquidityPool(
	ctx context.Context,
	mu state.Mutable,
	liquidityPoolAddress codec.Address,
	functionID uint64,
	tokenX codec.Address,
	tokenY codec.Address,
	fee uint64,
	reserveX uint64,
	reserveY uint64,
	liquidityToken codec.Address,
) error {
	k := LiquidityPoolKey(liquidityPoolAddress)
	v := make([]byte, lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len+lconsts.Uint64Len+lconsts.Uint64Len+codec.AddressLen)
	binary.BigEndian.PutUint64(v, functionID)
	copy(v[lconsts.Uint64Len:], tokenX[:])
	copy(v[lconsts.Uint64Len+codec.AddressLen:], tokenY[:])
	binary.BigEndian.PutUint64(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen:], fee)
	binary.BigEndian.PutUint64(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len:], reserveX)
	binary.BigEndian.PutUint64(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len+lconsts.Uint64Len:], reserveY)
	copy(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len+lconsts.Uint64Len+lconsts.Uint64Len:], liquidityToken[:])
	return mu.Insert(ctx, k, v)
}

func GetLiquidityPoolNoController(
	ctx context.Context,
	mu state.Immutable,
	poolAddress codec.Address,
) (uint64, codec.Address, codec.Address, uint64, uint64, uint64, codec.Address, error) {
	k := LiquidityPoolKey(poolAddress)
	v, err := mu.GetValue(ctx, k)
	if err != nil {
		return 0, codec.EmptyAddress, codec.EmptyAddress, 0, 0, 0, codec.EmptyAddress, err
	}
	return innerGetLiquidityPool(v)
}

func innerGetLiquidityPool(
	v []byte,
) (uint64, codec.Address, codec.Address, uint64, uint64, uint64, codec.Address, error) {
	functionID := binary.BigEndian.Uint64(v)
	tokenX := codec.Address(v[lconsts.Uint64Len : lconsts.Uint64Len+codec.AddressLen])
	tokenY := codec.Address(v[lconsts.Uint64Len+codec.AddressLen : lconsts.Uint64Len+codec.AddressLen+codec.AddressLen])
	fee := binary.BigEndian.Uint64(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen:])
	reserveX := binary.BigEndian.Uint64(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len:])
	reserveY := binary.BigEndian.Uint64(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len+lconsts.Uint64Len:])
	lpTokenAddress := codec.Address(v[lconsts.Uint64Len+codec.AddressLen+codec.AddressLen+lconsts.Uint64Len+lconsts.Uint64Len+lconsts.Uint64Len:])
	return functionID, tokenX, tokenY, fee, reserveX, reserveY, lpTokenAddress, nil
}

func CompareAddress(a codec.Address, b codec.Address) ComparisonValue {
	// TODO: make more idiomatic
	x := make([]int, codec.AddressLen)
	for i := range x {
		if a[i] < b[i] {
			return LessThan
		} else if a[i] > b[i] {
			return GreaterThan
		}
	}
	return Equal
}
