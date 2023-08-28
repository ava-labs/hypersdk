// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	mapModuleName = "map"
	mapOk         = 0
	mapErr        = -1
)

type maps map[string][]byte

// Key value store for program data
type storage struct {
	// int64 for simplicity, could be a real hash later
	state    map[int64]maps
	counter  int64
	Programs map[uint32][]byte
}

type MapModule struct {
	meter Meter
	log   logging.Logger
}

// NewMapModule returns a new map host module which can manage in memory state.
// This is a placeholder storage system intended to show how a wasm program
// would access/modify persistent storage.
func NewMapModule(log logging.Logger, meter Meter) *MapModule {
	return &MapModule{
		meter: meter,
		log:   log,
	}
}

// GlobalStorage is a global variable that holds the state of all programs.
// This is a placeholder storage system intended to show how a wasm program would access/modify persistent storage.
// Needs to be global, so state can be persisted across multiple runtime intances.
var GlobalStorage = storage{
	state:    make(map[int64]maps),
	counter:  0,
	Programs: make(map[uint32][]byte),
}

func (m *MapModule) Instantiate(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder(mapModuleName).
		NewFunctionBuilder().WithFunc(initializeFn).Export("init_program").
		NewFunctionBuilder().WithFunc(m.storeBytesFn).Export("store_bytes").
		NewFunctionBuilder().WithFunc(m.getBytesLenFn).Export("get_bytes_len").
		NewFunctionBuilder().WithFunc(m.getBytesFn).Export("get_bytes").
		Instantiate(ctx)

	return err
}

func initializeFn() int64 {
	GlobalStorage.counter++
	GlobalStorage.state[GlobalStorage.counter] = make(map[string][]byte)
	return GlobalStorage.counter
}

func (m *MapModule) storeBytesFn(_ context.Context, mod api.Module, id int64, keyPtr uint32, keyLength uint32, valuePtr uint32, valueLength uint32) int32 {
	_, ok := GlobalStorage.state[id]
	if !ok {
		m.log.Error("failed to find program id in storage")
		return mapErr
	}

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
	GlobalStorage.state[id][string(keyBuf)] = copiedValue

	return mapOk
}

func (m *MapModule) getBytesLenFn(_ context.Context, mod api.Module, id int64, keyPtr uint32, keyLength uint32) int32 {
	_, ok := GlobalStorage.state[id]
	if !ok {
		m.log.Error("failed to find program id in storage")
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, keyLength)
	if !ok {
		return mapErr
	}
	val, ok := GlobalStorage.state[id][string(buf)]
	if !ok {
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
	_, ok := GlobalStorage.state[id]
	if !ok {
		m.log.Error("failed to find program id in storage")
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, uint32(keyLength))
	if !ok {
		return mapErr
	}
	val, ok := GlobalStorage.state[id][string(buf)]
	if !ok {
		return mapErr
	}

	result, err := mod.ExportedFunction("alloc").Call(ctx, uint64(valLength))
	if err != nil {
		m.log.Error("failed to allocate memory for value: %v", zap.Error(err))
		return mapErr
	}
	ptr := result[0]
	// write to memory
	if !mod.Memory().Write(uint32(ptr), val) {
		m.log.Error("failed to write value to memory")
		return mapErr
	}

	return int32(ptr)
}
