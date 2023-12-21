// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/engine"
)

var _ Runtime = &WasmRuntime{}

// New returns a new wasm runtime.
func New(
	log logging.Logger,
	engine *engine.Engine,
	imports SupportedImports,
	cfg *Config,
) Runtime {
	return &WasmRuntime{
		log:     log,
		engine:  engine,
		imports: imports,
		cfg:     cfg,
	}
}

type WasmRuntime struct {
	engine *engine.Engine
	store  *engine.Store
	inst   *wasmtime.Instance
	mod    *wasmtime.Module
	exp    WasmtimeExportClient
	meter  *engine.Meter
	cfg    *Config

	once     sync.Once
	cancelFn context.CancelFunc

	imports SupportedImports

	log logging.Logger
}

func (r *WasmRuntime) Initialize(ctx context.Context, programBytes []byte, maxUnits uint64) (err error) {
	ctx, r.cancelFn = context.WithCancel(ctx)
	go func(ctx context.Context) {
		<-ctx.Done()
		// send immediate interrupt to engine
		r.Stop()
	}(ctx)

	// create store
	cfg := engine.NewStoreConfig()
	cfg.SetLimitMaxMemory(r.cfg.LimitMaxMemory)
	r.store = engine.NewStore(r.engine, cfg)

	// enable wasi logging support only in debug mode
	if r.cfg.EnableDebugMode {
		wasiConfig := wasmtime.NewWasiConfig()
		wasiConfig.InheritStderr()
		wasiConfig.InheritStdout()
		r.store.SetWasi(wasiConfig)
	}

	// add metered units to the store
	err = r.store.AddUnits(maxUnits)
	if err != nil {
		return err
	}

	// create module
	r.mod, err = engine.NewModule(r.engine, programBytes, r.cfg.CompileStrategy)
	if err != nil {
		return err
	}

	// create linker
	link := Link{wasmtime.NewLinker(r.store.GetEngine())}

	// enable wasi logging support only in testing/debug mode
	if r.cfg.EnableDebugMode {
		wasiConfig := wasmtime.NewWasiConfig()
		wasiConfig.InheritStderr()
		wasiConfig.InheritStdout()
		r.store.SetWasi(wasiConfig)
		err = link.DefineWasi()
		if err != nil {
			return err
		}
	}

	// setup metering
	r.meter, err = engine.NewMeter(r.store)
	if err != nil {
		return err
	}

	// setup client capable of calling exported functions
	r.exp = newExportClient(r.inst, r.store.Get())

	imports := getRegisteredImportModules(r.mod.Imports())
	// register host functions exposed to the guest (imports)
	for _, imp := range imports {
		// registered separately by linker
		mod, ok := r.imports[imp]
		if !ok {
			return fmt.Errorf("%w: %s", ErrMissingImportModule, imp)
		}
		err = mod().Register(link, r.meter, r.imports)
		if err != nil {
			return err
		}
	}

	// instantiate the module with all of the imports defined by the linker
	r.inst, err = link.Instantiate(r.store.Get(), r.mod)
	if err != nil {
		return err
	}

	return nil
}

// getRegisteredImportModules returns the unique names of all import modules registered
// by the wasm module.
func getRegisteredImportModules(importTypes []*wasmtime.ImportType) []string {
	u := make(map[string]struct{}, len(importTypes))
	imports := []string{}
	for _, t := range importTypes {
		mod := t.Module()
		if mod == wasiPreview1ModName {
			continue
		}
		if _, ok := u[mod]; ok {
			continue
		}
		u[mod] = struct{}{}
		imports = append(imports, mod)
	}
	return imports
}

func (r *WasmRuntime) Call(_ context.Context, name string, params ...SmartPtr) ([]int64, error) {
	var fnName string
	switch name {
	case AllocFnName, DeallocFnName, MemoryFnName:
		fnName = name
	default:
		// the SDK will append the guest suffix to the function name
		fnName = name + guestSuffix
	}

	fn := r.inst.GetFunc(r.store.Get(), fnName)
	if fn == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, name)
	}

	fnParams := fn.Type(r.store.Get()).Params()
	if len(params) != len(fnParams) {
		return nil, fmt.Errorf("%w for function %s: %d expected: %d", ErrInvalidParamCount, name, len(params), len(fnParams))
	}

	callParams, err := mapFunctionParams(params, fnParams)
	if err != nil {
		return nil, err
	}

	result, err := fn.Call(r.store.Get(), callParams...)
	if err != nil {
		return nil, fmt.Errorf("export function call failed %s: %w", name, handleTrapError(err))
	}

	switch v := result.(type) {
	case int32:
		value := int64(result.(int32))
		return []int64{value}, nil
	case int64:
		value := result.(int64)
		return []int64{value}, nil
	case nil:
		// the function had no return values
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid result type: %v", v)
	}
}

func (r *WasmRuntime) Memory() Memory {
	return NewMemory(newExportClient(r.inst, r.store.Get()))
}

func (r *WasmRuntime) Meter() *engine.Meter {
	return r.meter
}

func (r *WasmRuntime) Stop() {
	r.once.Do(func() {
		r.log.Debug("shutting down runtime engine...")
		// send immediate interrupt to engine and all children stores.
		r.engine.Stop()
		r.cancelFn()
	})
}

// mapFunctionParams maps call input to the expected wasm function params.
func mapFunctionParams(input []SmartPtr, values []*wasmtime.ValType) ([]interface{}, error) {
	params := make([]interface{}, len(values))
	for i, v := range values {
		switch v.Kind() {
		case wasmtime.KindI32:
			// ensure this value is within the range of an int32
			if !EnsureIntToInt32(int(input[i])) {
				return nil, fmt.Errorf("%w: %d", ErrIntegerConversionOverflow, input[i])
			}
			params[i] = int32(input[i])
		case wasmtime.KindI64:
			params[i] = int64(input[i])
		default:
			return nil, fmt.Errorf("%w: %v", ErrInvalidParamType, v.Kind())
		}
	}

	return params, nil
}
