// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

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
	// uint64 for simplicity, could be a real hash later
	state   map[uint64]maps
	mods    map[uint64]api.Module
	counter uint64
}

type MapModule struct {
	meter Meter
	log   logging.Logger
	store storage
}

// NewMapModule returns a new map host module which can manage in memory state.
// This is a placeholder storage system intended to show how a wasm program
// would access/modify persistent storage.
func NewMapModule(log logging.Logger, meter Meter) *MapModule {
	return &MapModule{
		meter: meter,
		log:   log,
		store: storage{
			state:   make(map[uint64]maps),
			mods:    make(map[uint64]api.Module),
			counter: 0,
		},
	}
}

func (m *MapModule) Instantiate(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder(mapModuleName).
		NewFunctionBuilder().WithFunc(m.initializeFn).Export("init_program").
		NewFunctionBuilder().WithFunc(m.storeBytesFn).Export("store_bytes").
		NewFunctionBuilder().WithFunc(m.getBytesLenFn).Export("get_bytes_len").
		NewFunctionBuilder().WithFunc(m.getBytesFn).Export("get_bytes").
		Instantiate(ctx)

	return err
}

func (m *MapModule) initializeFn(_ context.Context, mod api.Module) uint64 {
	m.store.counter++
	m.store.state[m.store.counter] = make(map[string][]byte)
	m.store.mods[m.store.counter] = mod
	return m.store.counter
}

func (m *MapModule) storeBytesFn(_ context.Context, mod api.Module, id uint64, keyPtr uint32, keyLength uint32, valuePtr uint32, valueLength uint32) int32 {
	_, ok := m.store.state[id]
	if !ok {
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

	m.store.state[id][string(keyBuf)] = copiedValue
	return mapOk
}

func (m *MapModule) getBytesLenFn(_ context.Context, mod api.Module, id uint64, keyPtr uint32, keyLength uint32) int32 {
	_, ok := m.store.state[id]
	if !ok {
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, keyLength)
	if !ok {
		return mapErr
	}
	val, ok := m.store.state[id][string(buf)]
	if !ok {
		return mapErr
	}
	return int32(len(val))
}

func (m *MapModule) getBytesFn(ctx context.Context, mod api.Module, id uint64, keyPtr uint32, keyLength int32, valLength int32) int32 {
	// Ensure the key and value lengths are positive
	if valLength < 0 || keyLength < 0 {
		return mapErr
	}
	_, ok := m.store.state[id]
	if !ok {
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, uint32(keyLength))
	if !ok {
		return mapErr
	}
	val, ok := m.store.state[id][string(buf)]
	if !ok {
		return mapErr
	}

	result, err := mod.ExportedFunction("alloc").Call(ctx, uint64(valLength))
	if err != nil {
		return mapErr
	}
	ptr := result[0]
	// write to memory
	if !mod.Memory().Write(uint32(ptr), val) {
		return mapErr
	}

	return int32(ptr)
}
