// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	// "encoding/binary"
	"errors"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v12"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	allocFnName   = "alloc"
	deallocFnName = "dealloc"
)

func NewWasmtime(log logging.Logger, meter Meter, storage Storage) *runtime {
	return &runtime{
		log:     log,
		meter:   meter,
		storage: storage,
	}
}

type runtime struct {
	cancelFn context.CancelFunc
	mod      *wasmtime.Instance
	store    *wasmtime.Store
	meter    Meter
	storage  Storage

	closed bool
	log    logging.Logger
}

func (r *runtime) Create(ctx context.Context, programBytes []byte) (uint64, error) {
	err := r.Initialize(ctx, programBytes)
	if err != nil {
		return 0, err
	}
	// get programId
	programID := InitProgramStorage()

	// call initialize if it exists
	result, err := r.Call(ctx, "init", uint64(programID))
	if err != nil {
		if !errors.Is(err, ErrMissingExportedFunction) {
			return 0, err
		}
	} else {
		// check boolean result from init
		if result[0] == 0 {
			return 0, fmt.Errorf("failed to initialize program")
		}
	}
	return uint64(programID), nil
}

func (r *runtime) Initialize(ctx context.Context, programBytes []byte) error {
	cfg := wasmtime.NewConfig()
	cfg.SetConsumeFuel(true)
	cfg.CacheConfigLoadDefault()
	cfg.SetStrategy(wasmtime.StrategyCranelift)
	r.store = wasmtime.NewStore(wasmtime.NewEngineWithConfig(cfg))
	err := r.store.AddFuel(10000000000000) // testing only ;)
	if err != nil {
		return err
	}

	module, err := wasmtime.NewModule(r.store.Engine, programBytes)
	if err != nil {
		return err
	}

	linker := wasmtime.NewLinker(r.store.Engine)

	// TODO: remove
	// initialize wasi
	wcfg := wasmtime.NewWasiConfig()
	wcfg.InheritStderr()
	wcfg.InheritStdout()
	r.store.SetWasi(wcfg)

	err = linker.DefineWasi()
	if err != nil {
		return err
	}

	mapMod := NewMapModule(r.log, nil)

	linker.DefineFunc(r.store, "map", "store_bytes", mapMod.storeBytesWasmtimeFn)
	linker.DefineFunc(r.store, "map", "get_bytes_len", mapMod.getBytesLenWasmtimeFn)
	linker.DefineFunc(r.store, "map", "get_bytes", mapMod.getBytesWasmtimeFn)

	// the exported 'memory' function is injected into WASM during the compilation.

	r.mod, err = linker.Instantiate(r.store, module)
	if err != nil {
		return err
	}

	mapMod.Mod = r.mod
	mapMod.Store = r.store

	return nil
}

func (r *runtime) Call(ctx context.Context, name string, params ...uint64) ([]uint64, error) {
	if r.closed {
		return nil, fmt.Errorf("failed to call: %s: runtime closed", name)
	}

	var api *wasmtime.Func

	if name == allocFnName || name == deallocFnName {
		api = r.mod.GetFunc(r.store, name)
	} else {
		api = r.mod.GetFunc(r.store, utils.GetGuestFnName(name))
	}

	if api == nil {
		return nil, ErrMissingExportedFunction
	}

	// why []interface....
	var p []interface{}
	for _, param := range params {
		p = append(p, int64(param))
	}

	result, err := api.Call(r.store, p...)
	if err != nil {
		return nil, fmt.Errorf("failed to call %s: %w", name, err)
	}

	switch v := result.(type) {
	case int32:
		return []uint64{uint64(result.(int32))}, nil
	case int64:
		return []uint64{uint64(result.(int64))}, nil
	default:
		return nil, fmt.Errorf("invalid type %v", v)
	}
}

func (r *runtime) GetGuestBuffer(offset uint32, length uint32) ([]byte, bool) {
	return utils.GetBufferWasmtime(r.mod, r.store, offset, length)
}

// TODO: ensure deallocate on Stop.
func (r *runtime) WriteGuestBuffer(ctx context.Context, buf []byte) (uint64, error) {
	// allocate to guest and return offset
	api := r.mod.GetFunc(r.store, allocFnName)
	if api == nil {
		return 0, ErrMissingExportedFunction
	}
	result, err := api.Call(r.store, len(buf))
	if err != nil {
		return 0, err
	}

	mem := r.mod.GetExport(r.store, "memory").Memory().UnsafeData(r.store)
	addr := result.(int32)
	// binary.BigEndian.PutUint32(mem[addr:], uint32(len(buf)))
	copy(mem[addr:], buf)

	return uint64(addr), nil
}

func (r *runtime) Stop(ctx context.Context) error {
	defer r.cancelFn()
	if r.closed {
		return nil
	}
	r.closed = true

	return nil
}
