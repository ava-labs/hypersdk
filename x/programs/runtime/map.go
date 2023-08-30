// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	mapModuleName = "map"
	mapOk         = 0
	mapErr        = -1
)

type MapModule struct {
	meter Meter
	log   logging.Logger
	db    database.Database
}

// NewMapModule returns a new map host module which can manage in memory state.
func NewMapModule(log logging.Logger, meter Meter, db database.Database) *MapModule {
	return &MapModule{
		meter: meter,
		log:   log,
		db:    db,
	}
}

func (m *MapModule) Instantiate(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder(mapModuleName).
		NewFunctionBuilder().WithFunc(m.storeBytesFn).Export("store_bytes").
		NewFunctionBuilder().WithFunc(m.getBytesLenFn).Export("get_bytes_len").
		NewFunctionBuilder().WithFunc(m.getBytesFn).Export("get_bytes").
		Instantiate(ctx)

	return err
}

func (m *MapModule) storeBytesFn(_ context.Context, mod api.Module, id int64, keyPtr uint32, keyLength uint32, valuePtr uint32, valueLength uint32) int32 {
	keyBuf, ok := utils.GetBuffer(mod, keyPtr, keyLength)
	if !ok {
		return mapErr
	}

	valBuf, ok := utils.GetBuffer(mod, valuePtr, valueLength)
	if !ok {
		return mapErr
	}

	// Need to copy the value because the GC can collect the value after this function returns
	copiedValue := make([]byte, len(valBuf))
	copy(copiedValue, valBuf)

	err := SetValue(m.db, uint64(id), keyBuf, copiedValue)
	if err != nil {
		m.log.Error("failed to set value in database")
		return mapErr
	}

	return mapOk
}

func (m *MapModule) getBytesLenFn(_ context.Context, mod api.Module, id int64, keyPtr uint32, keyLength uint32) int32 {
	buf, ok := utils.GetBuffer(mod, keyPtr, keyLength)
	if !ok {
		return mapErr
	}
	val, err := GetValue(m.db, uint64(id), buf)
	if err != nil {
		return mapErr
	}
	return int32(len(val))
}

func (m *MapModule) getBytesFn(ctx context.Context, mod api.Module, id int64, keyPtr uint32, keyLength int32, valLength int32) int32 {
	// Ensure the key and value lengths are positive
	if valLength < 0 || keyLength < 0 {
		m.log.Error("key or value length is negative")
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, uint32(keyLength))
	if !ok {
		return mapErr
	}

	// Get the value from the database
	val, err := GetValue(m.db, uint64(id), buf)
	if err != nil {
		return mapErr
	}

	ptr, err := utils.WriteBuffer(ctx, mod, val)
	if err != nil {
		m.log.Error("failed to write value to memory")
		return mapErr
	}

	return int32(ptr)
}
