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
	programPrefix        = 0x0
	ProgramChunks uint16 = 1
)

func ProgramPrefixKey(id []byte, key []byte) (k []byte) {
	k = make([]byte, codec.LIDLen+1+len(key))
	k[0] = programPrefix
	copy(k, id)
	copy(k[codec.LIDLen:], (key))
	return
}

//
// Program
//

func ProgramKey(id codec.LID) (k []byte) {
	k = make([]byte, 1+codec.LIDLen)
	copy(k[1:], id[:])
	return
}

// [programID] -> [programBytes]
func GetProgram(
	ctx context.Context,
	db state.Immutable,
	programID codec.LID,
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
	programID codec.LID,
	program []byte,
) error {
	k := ProgramKey(programID)
	return mu.Insert(ctx, k, program)
}
