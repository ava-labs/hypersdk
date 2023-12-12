// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

const (
	programPrefix = 0x0
)

// ProgramPrefixKey returns a properly formatted key
// for storing a value at [address][key].
func ProgramPrefixKey(address []byte, key []byte) (k []byte) {
	k = make([]byte, codec.AddressLen+1+len(key))
	k[0] = programPrefix
	copy(k, address[:])
	copy(k[codec.AddressLen:], (key[:]))
	return
}

//
// Program
//

// ProgramKey returns the key used to store the program bytes at [address].
func ProgramKey(address codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen)
	copy(k[1:], address[:])
	k[0] = programPrefix
	return
}

// GetProgram returns the programBytes stored at [programAddress].
func GetProgram(
	ctx context.Context,
	db state.Immutable,
	programAddress codec.Address,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	k := ProgramKey(programAddress)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// SetProgram stores [program] at [programAddress]
func SetProgram(
	ctx context.Context,
	mu state.Mutable,
	programAddress codec.Address,
	program []byte,
) error {
	k := ProgramKey(programAddress)
	return mu.Insert(ctx, k, program)
}
