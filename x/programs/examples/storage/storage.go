// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

var ErrInvalidBalance = errors.New("invalid balance")

const (
	balancePrefix = 0x0
	programPrefix = 0x1
)

const (
	BalanceChunks uint16 = 1
	ProgramChunks uint16 = 512 // 32KiB/64B
)

func ProgramPrefixKey(id []byte, key []byte) (k []byte) {
	k = make([]byte, consts.IDLen+1+len(key))
	k[0] = programPrefix
	copy(k, id[:])
	copy(k[consts.IDLen:], (key[:]))
	return
}

//
// Program
//

func ProgramKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	copy(k[1:], id[:])
	binary.BigEndian.PutUint16(k[1+consts.IDLen:], ProgramChunks)
	return
}

// [programID] -> [programBytes]
func GetProgram(
	ctx context.Context,
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	k := ProgramKey(programID)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// SetProgram stores [program] at [programID]
func SetProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	program []byte,
) error {
	k := ProgramKey(programID)
	return mu.Insert(ctx, k, program)
}

//
// Balance
//

// [balancePrefix] + [address]
func BalanceKey(addr codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = balancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
	return
}
