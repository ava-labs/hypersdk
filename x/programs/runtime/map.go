package runtime

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/meter"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	mapModuleName = "map"
	mapOk         = 0
	mapErr        = -1
)

type maps map[string][]byte

// Represents a placeholder storage system intended to show how a wasm program would
// access/modify persistent storage.
type storage struct {
	// give a program id a key value store for its data
	// (use a uint64 for simplicity, could be a real hash later)
	State   map[uint64]maps
	Mods    map[uint64]api.Module
	Counter uint64
}

var store = &storage{
	State:   make(map[uint64]maps),
	Mods:    make(map[uint64]api.Module),
	Counter: 0,
}

type MapModule struct {
	meter meter.Meter
	log   logging.Logger
}

func NewMapModule(log logging.Logger, meter meter.Meter) *MapModule {
	return &MapModule{
		meter: meter,
		log:   log,
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
	store.Counter++
	store.State[store.Counter] = make(map[string][]byte)
	store.Mods[store.Counter] = mod
	return store.Counter
}

func (m *MapModule) storeBytesFn(_ context.Context, mod api.Module, ID uint64, keyPtr uint32, keyLength uint32, valuePtr uint32, valueLength uint32) int32 {
	_, ok := store.State[ID]
	if !ok {
		return mapOk
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

	store.State[ID][string(keyBuf)] = copiedValue
	return mapOk
}

func (m *MapModule) getBytesLenFn(_ context.Context, mod api.Module, ID uint64, keyPtr uint32, keyLength uint32) int32 {
	_, ok := store.State[ID]
	if !ok {
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, keyLength)
	if !ok {
		return mapErr
	}
	val, ok := store.State[ID][string(buf)]
	if !ok {
		return mapErr
	}
	return int32(len(val))
}

// Returns a pointer to the value
func (m *MapModule) getBytesFn(ctx context.Context, mod api.Module, ID uint64, keyPtr uint32, keyLength uint32, valLength int32) int32 {
	_, ok := store.State[ID]
	if !ok {
		return mapErr
	}
	buf, ok := utils.GetBuffer(mod, keyPtr, keyLength)
	if !ok {
		return mapErr
	}
	val, ok := store.State[ID][string(buf)]
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
