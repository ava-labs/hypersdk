// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	smath "github.com/ava-labs/avalanchego/utils/math"
	lconsts "github.com/ava-labs/hypersdk/consts"
)

// Note: to remove decimal, remember that it is 1 byte

const (
	MaxTokenNameSize     = 64
	MaxTokenSymbolSize   = 8
	MaxTokenMetadataSize = 256
	MaxTokenDecimals     = 18
)

func TokenInfoKey(tokenAddress codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+lconsts.Uint16Len)
	k[0] = tokenInfoPrefix
	copy(k[1:1+codec.AddressLen], tokenAddress[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], TokenInfoChunks)
	return k
}

func TokenAddress(name []byte, symbol []byte, metadata []byte) codec.Address {
	v := make([]byte, len(name)+len(symbol)+len(metadata))
	copy(v, name)
	copy(v[len(name):], symbol)
	copy(v[len(name)+len(symbol):], metadata)
	id := utils.ToID(v)
	return codec.CreateAddress(consts.TOKENID, id)
}

func TokenAccountKey(token codec.Address, account codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+codec.AddressLen+lconsts.Uint16Len)
	k[0] = tokenAccountInfoPrefix
	copy(k[1:], token[:])
	copy(k[1+codec.AddressLen:], account[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen+codec.AddressLen:], TokenAccountInfoChunks)
	return k
}

func SetTokenInfo(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	name []byte,
	symbol []byte,
	decimals uint8,
	metadata []byte,
	totalSupply uint64,
	owner codec.Address,
) error {
	// Setup
	k := TokenInfoKey(tokenAddress)
	nameLen := len(name)
	symbolLen := len(symbol)
	metadataLen := len(metadata)
	tokenInfoSize := lconsts.Uint16Len + nameLen + lconsts.Uint16Len + symbolLen + lconsts.Uint8Len + lconsts.Uint16Len + metadataLen + lconsts.Uint64Len + codec.AddressLen
	v := make([]byte, tokenInfoSize)

	// Insert name
	binary.BigEndian.PutUint16(v, uint16(nameLen))
	copy(v[lconsts.Uint16Len:lconsts.Uint16Len+nameLen], name)
	// Insert symbol
	binary.BigEndian.PutUint16(v[lconsts.Uint16Len+nameLen:], uint16(symbolLen))
	copy(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len:lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen], symbol)
	// Insert decimals
	v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen] = decimals
	// Insert metadata
	binary.BigEndian.PutUint16(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen:], uint16(metadataLen))
	copy(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len:lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len+metadataLen], metadata)
	// Insert totalSupply
	binary.BigEndian.PutUint64(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len+metadataLen:], totalSupply)
	// Insert owner
	copy(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len+metadataLen+lconsts.Uint64Len:], owner[:])
	return mu.Insert(ctx, k, v)
}

func GetTokenInfo(
	ctx context.Context,
	f ReadState,
	tokenAddress codec.Address,
) ([]byte, []byte, uint8, []byte, uint64, codec.Address, error) {
	k := TokenInfoKey(tokenAddress)
	values, errs := f(ctx, [][]byte{k})
	if errs[0] != nil {
		return nil, nil, 0, nil, 0, codec.EmptyAddress, errs[0]
	}
	return innerGetTokenInfo(values[0])
}

func GetTokenInfoNoController(
	ctx context.Context,
	mu state.Immutable,
	tokenAddress codec.Address,
) ([]byte, []byte, uint8, []byte, uint64, codec.Address, error) {
	k := TokenInfoKey(tokenAddress)
	v, err := mu.GetValue(ctx, k)
	if err != nil {
		return nil, nil, 0, nil, 0, codec.EmptyAddress, err
	}
	return innerGetTokenInfo(v)
}

func innerGetTokenInfo(
	v []byte,
) ([]byte, []byte, uint8, []byte, uint64, codec.Address, error) {
	// Extract name
	nameLen := binary.BigEndian.Uint16(v)
	name := v[lconsts.Uint16Len : lconsts.Uint16Len+nameLen]
	// Extract symbol
	symbolLen := binary.BigEndian.Uint16(v[lconsts.Uint16Len+nameLen:])
	symbol := v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len : lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen]
	// Extract decimals
	decimals := v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen]
	// Extract metadata
	metadataLen := binary.BigEndian.Uint16(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen:])
	metadata := v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len : lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len+metadataLen]
	// Extract totalSupply
	totalSupply := binary.BigEndian.Uint64(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len+metadataLen:])
	// Extract owner
	owner := codec.Address(v[lconsts.Uint16Len+nameLen+lconsts.Uint16Len+symbolLen+lconsts.ByteLen+lconsts.Uint16Len+metadataLen+lconsts.Uint64Len:])

	return name, symbol, decimals, metadata, totalSupply, owner, nil
}

func SetTokenAccount(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	account codec.Address,
	balance uint64,
) error {
	k := TokenAccountKey(tokenAddress, account)
	v := make([]byte, lconsts.Uint64Len)
	binary.BigEndian.PutUint64(v, balance)
	return mu.Insert(ctx, k, v)
}

// This function updates both the token info state and the token account state
// Note: it is the responsibility of the user to check that invariants related
// to this function is met, as this function does not nicely wrap any errors
// that may arise
func MintToken(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	to codec.Address,
	mintAmount uint64,
) error {
	// Get token info + account
	tName, tSymbol, tDecimals, tMetadata, tSupply, tOwner, err := GetTokenInfoNoController(ctx, mu, tokenAddress)
	if err != nil {
		return err
	}
	balance, err := GetTokenAccountNoController(ctx, mu, tokenAddress, to)
	if err != nil {
		return err
	}
	newTotalSupply, err := smath.Add64(tSupply, mintAmount)
	if err != nil {
		return err
	}
	newBalance, err := smath.Add64(balance, mintAmount)
	if err != nil {
		return err
	}
	// Update token info
	if err := SetTokenInfo(ctx, mu, tokenAddress, tName, tSymbol, tDecimals, tMetadata, newTotalSupply, tOwner); err != nil {
		return err
	}
	// Update token account
	if err := SetTokenAccount(ctx, mu, tokenAddress, to, newBalance); err != nil {
		return err
	}
	return nil
}

// Note: it is the responsibility of the user to check that invariants related
// to this function is met, as this function does not nicely wrap any errors
// that may arise
func TransferToken(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	from codec.Address,
	to codec.Address,
	value uint64,
) error {
	fromBalance, err := GetTokenAccountNoController(ctx, mu, tokenAddress, from)
	if err != nil {
		return err
	}
	toBalance, err := GetTokenAccountNoController(ctx, mu, tokenAddress, to)
	if err != nil {
		return err
	}
	newFromBalance, err := smath.Sub(fromBalance, value)
	if err != nil {
		return err
	}
	newToBalance, err := smath.Add64(toBalance, value)
	if err != nil {
		return err
	}
	if err = SetTokenAccount(ctx, mu, tokenAddress, from, newFromBalance); err != nil {
		return err
	}
	if err = SetTokenAccount(ctx, mu, tokenAddress, to, newToBalance); err != nil {
		return err
	}
	return nil
}

// Function that burns token by updating both token info and token account
// Note: it is the responsibility of the user to check that invariants related
// to this function is met, as this function does not nicely wrap any errors
// that may arise
func BurnToken(
	ctx context.Context,
	mu state.Mutable,
	tokenAddress codec.Address,
	from codec.Address,
	value uint64,
) error {
	balance, err := GetTokenAccountNoController(ctx, mu, tokenAddress, from)
	if err != nil {
		return err
	}
	name, symbol, decimals, metadata, totalSupply, owner, err := GetTokenInfoNoController(ctx, mu, tokenAddress)
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

	if err = SetTokenAccount(ctx, mu, tokenAddress, from, newBalance); err != nil {
		return err
	}
	if err = SetTokenInfo(ctx, mu, tokenAddress, name, symbol, decimals, metadata, newTotalSupply, owner); err != nil {
		return err
	}

	return nil
}

func GetTokenAccount(
	ctx context.Context,
	f ReadState,
	tokenAddress codec.Address,
	account codec.Address,
) (uint64, error) {
	k := TokenAccountKey(tokenAddress, account)
	values, errs := f(ctx, [][]byte{k})
	if errs[0] == database.ErrNotFound {
		return 0, nil
	}
	if errs[0] != nil {
		return 0, errs[0]
	}
	return binary.BigEndian.Uint64(values[0]), nil
}

func GetTokenAccountNoController(
	ctx context.Context,
	mu state.Immutable,
	tokenAddress codec.Address,
	account codec.Address,
) (uint64, error) {
	k := TokenAccountKey(tokenAddress, account)
	v, err := mu.GetValue(ctx, k)
	if err == database.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(v), nil
}
