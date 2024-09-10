// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	smath "github.com/ava-labs/avalanchego/utils/math"
	hconsts "github.com/ava-labs/hypersdk/consts"
)

func TokenAddress(name []byte, symbol []byte, metadata []byte) codec.Address {
	v := make([]byte, len(name)+len(symbol)+len(metadata))
	copy(v, name)
	copy(v[len(name):], symbol)
	copy(v[len(name)+len(symbol):], metadata)
	id := utils.ToID(v)
	return codec.CreateAddress(consts.TOKENID, id)
}

func TokenInfoKey(tokenAddress codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+hconsts.Uint16Len)
	k[0] = tokenInfoPrefix
	copy(k[1:1+codec.AddressLen], tokenAddress[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], TokenInfoChunks)
	return k
}

func TokenAccountBalanceKey(token codec.Address, account codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+codec.AddressLen+hconsts.Uint16Len)
	k[0] = tokenAccountBalancePrefix
	copy(k[1:], token[:])
	copy(k[1+codec.AddressLen:], account[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen+codec.AddressLen:], TokenAccountBalanceChunks)
	return k
}

func SetTokenInfo(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	name []byte,
	symbol []byte,
	metadata []byte,
	totalSupply uint64,
	owner codec.Address,
) error {
	// Setup
	k := TokenInfoKey(tokenAddress)
	nameLen := len(name)
	symbolLen := len(symbol)
	metadataLen := len(metadata)
	tokenInfoSize := hconsts.Uint16Len + nameLen + hconsts.Uint16Len + symbolLen + hconsts.Uint16Len + metadataLen + hconsts.Uint64Len + codec.AddressLen
	v := make([]byte, tokenInfoSize)

	// Insert name
	binary.BigEndian.PutUint16(v, uint16(nameLen))
	copy(v[hconsts.Uint16Len:hconsts.Uint16Len+nameLen], name)
	// Insert symbol
	binary.BigEndian.PutUint16(v[hconsts.Uint16Len+nameLen:], uint16(symbolLen))
	copy(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len:hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen], symbol)
	// Insert metadata
	binary.BigEndian.PutUint16(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen:], uint16(metadataLen))
	copy(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len:hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len+metadataLen], metadata)
	// Insert totalSupply
	binary.BigEndian.PutUint64(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len+metadataLen:], totalSupply)
	// Insert owner
	copy(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len+metadataLen+hconsts.Uint64Len:], owner[:])
	return mu.Insert(ctx, k, v)
}

func GetTokenInfoNoController(
	ctx context.Context,
	mu state.Immutable,
	tokenAddress codec.Address,
) ([]byte, []byte, []byte, uint64, codec.Address, error) {
	k := TokenInfoKey(tokenAddress)
	v, err := mu.GetValue(ctx, k)
	if err != nil {
		return nil, nil, nil, 0, codec.EmptyAddress, err
	}
	return innerGetTokenInfo(v)
}

func innerGetTokenInfo(
	v []byte,
) ([]byte, []byte, []byte, uint64, codec.Address, error) {
	// Extract name
	nameLen := binary.BigEndian.Uint16(v)
	name := v[hconsts.Uint16Len : hconsts.Uint16Len+nameLen]
	// Extract symbol
	symbolLen := binary.BigEndian.Uint16(v[hconsts.Uint16Len+nameLen:])
	symbol := v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len : hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen]
	// Extract metadata
	metadataLen := binary.BigEndian.Uint16(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen:])
	metadata := v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len : hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len+metadataLen]
	// Extract totalSupply
	totalSupply := binary.BigEndian.Uint64(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len+metadataLen:])
	// Extract owner
	owner := codec.Address(v[hconsts.Uint16Len+nameLen+hconsts.Uint16Len+symbolLen+hconsts.Uint16Len+metadataLen+hconsts.Uint64Len:])

	return name, symbol, metadata, totalSupply, owner, nil
}

func MintToken(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	to codec.Address,
	mintAmount uint64,
) error {
	// Get token info + account
	tName, tSymbol, tMetadata, tSupply, tOwner, err := GetTokenInfoNoController(ctx, mu, tokenAddress)
	if err != nil {
		return err
	}
	balance, err := GetTokenAccountBalanceNoController(ctx, mu, tokenAddress, to)
	if err != nil {
		return err
	}
	newTotalSupply, err := smath.Add(tSupply, mintAmount)
	if err != nil {
		return err
	}
	newBalance, err := smath.Add(balance, mintAmount)
	if err != nil {
		return err
	}
	// Update token info
	if err := SetTokenInfo(ctx, mu, tokenAddress, tName, tSymbol, tMetadata, newTotalSupply, tOwner); err != nil {
		return err
	}
	// Update token account
	if err := SetTokenAccountBalance(ctx, mu, tokenAddress, to, newBalance); err != nil {
		return err
	}
	return nil
}

func SetTokenAccountBalance(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	account codec.Address,
	balance uint64,
) error {
	k := TokenAccountBalanceKey(tokenAddress, account)
	v := make([]byte, hconsts.Uint64Len)
	binary.BigEndian.PutUint64(v, balance)
	return mu.Insert(ctx, k, v)
}

func GetTokenAccountBalanceNoController(
	ctx context.Context,
	mu state.Immutable,
	tokenAddress codec.Address,
	account codec.Address,
) (uint64, error) {
	k := TokenAccountBalanceKey(tokenAddress, account)
	v, err := mu.GetValue(ctx, k)
	if err == database.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(v), nil
}

func BurnToken(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	from codec.Address,
	value uint64,
) error {
	balance, err := GetTokenAccountBalanceNoController(ctx, mu, tokenAddress, from)
	if err != nil {
		return err
	}
	name, symbol, metadata, totalSupply, owner, err := GetTokenInfoNoController(ctx, mu, tokenAddress)
	if err != nil {
		return err
	}

	newBalance, err := smath.Sub(balance, value)
	if err != nil {
		return err
	}
	newTotalSupply, err := smath.Sub(totalSupply, value)
	if err != nil {
		return err
	}

	if err = SetTokenAccountBalance(ctx, mu, tokenAddress, from, newBalance); err != nil {
		return err
	}
	if err = SetTokenInfo(ctx, mu, tokenAddress, name, symbol, metadata, newTotalSupply, owner); err != nil {
		return err
	}

	return nil
}

func TransferToken(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	from codec.Address,
	to codec.Address,
	value uint64,
) error {
	fromBalance, err := GetTokenAccountBalanceNoController(ctx, mu, tokenAddress, from)
	if err != nil {
		return err
	}
	toBalance, err := GetTokenAccountBalanceNoController(ctx, mu, tokenAddress, to)
	if err != nil {
		return err
	}
	fmt.Println("from balance:", fromBalance, "to balance:", toBalance)
	newFromBalance, err := smath.Sub(fromBalance, value)
	if err != nil {
		return err
	}
	newToBalance, err := smath.Add(toBalance, value)
	if err != nil {
		return err
	}
	if err = SetTokenAccountBalance(ctx, mu, tokenAddress, from, newFromBalance); err != nil {
		return err
	}
	if err = SetTokenAccountBalance(ctx, mu, tokenAddress, to, newToBalance); err != nil {
		return err
	}
	return nil
}

func TokenExists(
	ctx context.Context,
	mu state.Immutable,
	tokenAddress codec.Address,
) bool {
	v, err := mu.GetValue(ctx, TokenInfoKey(tokenAddress))
	return v != nil && err == nil
}
