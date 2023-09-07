// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	HRP                = "simulator_key_"
	programPrefix      = 0x0
	programCountPrefix = 0x1
	storagePrefix      = 0x2
)

func ProgramKey(asset uint64) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	// convert uint64 to bytes
	binary.BigEndian.PutUint64(k[1:], asset)
	k[0] = programPrefix
	return
}

// StorageKey returns the key used to store a value in the program's storage
// [storagePrefix] + [programID] + [key]
func StorageKey(programID uint64, key []byte) (k []byte) {
	k = make([]byte, 1+consts.Int64Len+len(key))
	k[0] = storagePrefix
	binary.BigEndian.PutUint64(k[1:], programID)
	copy(k[1+consts.Int64Len:], key)
	return k
}

// ProgramCountKey returns the key used to store the program count
func ProgramCountKey() []byte {
	return []byte{programCountPrefix}
}

// IncrementProgramCount increments the program count by 1
func IncrementProgramCount(db database.Database) error {
	count, err := GetProgramCount(db)
	if err != nil {
		return err
	}
	count++

	countBytes := make([]byte, consts.Int64Len)
	binary.BigEndian.PutUint64(countBytes, count)
	err = db.Put(ProgramCountKey(), countBytes)
	return err
}

func GetProgramCount(db database.Database) (uint64, error) {
	countKey := ProgramCountKey()
	bytes, err := db.Get(countKey)
	if err != nil {
		// if not found, db hasn't been set up yet
		if errors.Is(err, database.ErrNotFound) {
			return 1, nil
		}
		return 0, err
	}

	count := binary.BigEndian.Uint64(bytes)
	return count, nil
}

// [programID] -> [programBytes]
func GetProgram(
	db database.Database,
	programID uint64,
) (
	bool, // exists
	[]byte, // program bytes
	error,
) {
	k := ProgramKey(programID)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, v, nil
}

// SetProgram stores [program] at [programID]
func SetProgram(
	db database.KeyValueWriter,
	programID uint64,
	program []byte,
) error {
	k := ProgramKey(programID)
	v := make([]byte, len(program))
	copy(v, program)
	return db.Put(k, v)
}

// GetValue returns the value stored at [key] in the [programID] storage
func GetValue(db database.Database, programID uint64, key []byte) ([]byte, error) {
	k := StorageKey(programID, key)
	value, err := db.Get(k)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// SetValue stores [value] at [key] in the [programID] storage
func SetValue(db database.KeyValueWriter, programID uint64, key []byte, value []byte) error {
	k := StorageKey(programID, key)
	return db.Put(k, value)
}
