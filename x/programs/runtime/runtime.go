// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/avalanchego/utils/logging"
)

var _ Runtime = &WasmRuntime{}

// New returns a new wasm runtime.
func New(log logging.Logger, cfg *Config, imports SupportedImports) Runtime {
	return &WasmRuntime{
		imports: imports,
		log:     log,
		cfg:     cfg,
	}
}

type WasmRuntime struct {
	cfg   *Config
	inst  *wasmtime.Instance
	store *wasmtime.Store
	mod   *wasmtime.Module
	exp   WasmtimeExportClient
	meter Meter

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

	engineConfig, err := r.cfg.Engine()
	if err != nil {
		return err
	}

	r.store = wasmtime.NewStore(wasmtime.NewEngineWithConfig(engineConfig))
	r.store.Limiter(
		r.cfg.limitMaxMemory,
		r.cfg.limitMaxTableElements,
		r.cfg.limitMaxInstances,
		r.cfg.limitMaxTables,
		r.cfg.limitMaxMemories,
	)

	// set initial epoch deadline
	r.store.SetEpochDeadline(1)

	switch r.cfg.compileStrategy {
	case PrecompiledWasm:
		// Note: that to deserialize successfully the bytes provided must have been
		// produced with an `Engine` that has the same compilation options as the
		// provided engine, and from the same version of this library.
		//
		// A precompile is not something we would store on chain.
		// Instead we would prefetch programs and precompile them.
		r.mod, err = wasmtime.NewModuleDeserialize(r.store.Engine, programBytes)
		if err != nil {
			return err
		}
	case CompileWasm:
		r.mod, err = wasmtime.NewModule(r.store.Engine, programBytes)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported compile strategy: %v", r.cfg.compileStrategy)
	}

	link := Link{wasmtime.NewLinker(r.store.Engine)}

	// enable wasi logging support only in testing/debug mode
	if r.cfg.debugMode {
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
	r.meter = NewMeter(r.store)
	_, err = r.meter.AddUnits(maxUnits)
	if err != nil {
		return err
	}

	// setup client capable of calling exported functions
	r.exp = newExportClient(r.inst, r.store)

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
	r.inst, err = link.Instantiate(r.store, r.mod)
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

func (r *WasmRuntime) Call(_ context.Context, name string, params ...int64) ([]int64, error) {
	var fnName string
	switch name {
	case AllocFnName, DeallocFnName, MemoryFnName:
		fnName = name
	default:
		// the SDK will append the guest suffix to the function name
		fnName = name + guestSuffix
	}

	fn := r.inst.GetFunc(r.store, fnName)
	if fn == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, name)
	}

	fnParams := fn.Type(r.store).Params()
	if len(params) != len(fnParams) {
		return nil, fmt.Errorf("%w for function %s: %d expected: %d", ErrInvalidParamCount, name, len(params), len(fnParams))
	}

	callParams, err := mapFunctionParams(params, fnParams)
	if err != nil {
		return nil, err
	}

	result, err := fn.Call(r.store, callParams...)
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
	return NewMemory(newExportClient(r.inst, r.store))
}

func (r *WasmRuntime) Meter() Meter {
	return r.meter
}

func (r *WasmRuntime) Stop() {
	r.once.Do(func() {
		r.log.Debug("shutting down runtime engine...")
		// send immediate interrupt to engine
		r.store.Engine.IncrementEpoch()
		r.cancelFn()
	})
}

// PreCompileWasm returns a precompiled wasm module.
//
// Note: these bytes can be deserialized by an `Engine` that has the same version.
// For that reason precompiled wasm modules should not be stored on chain.
func PreCompileWasmBytes(programBytes []byte, cfg *Config) ([]byte, error) {
	engineConfig, err := cfg.Engine()
	if err != nil {
		return nil, err
	}

	store := wasmtime.NewStore(wasmtime.NewEngineWithConfig(engineConfig))
	store.Limiter(
		cfg.limitMaxMemory,
		cfg.limitMaxTableElements,
		cfg.limitMaxInstances,
		cfg.limitMaxTables,
		cfg.limitMaxMemories,
	)

	module, err := wasmtime.NewModule(store.Engine, programBytes)
	if err != nil {
		return nil, err
	}

	return module.Serialize()
}

// mapFunctionParams maps call input to the expected wasm function params.
func mapFunctionParams(input []int64, values []*wasmtime.ValType) ([]interface{}, error) {
	params := make([]interface{}, len(values))
	for i, v := range values {
		switch v.Kind() {
		case wasmtime.KindI32:
			// ensure this value is within the range of an int32
			if !EnsureInt64ToInt32(input[i]) {
				return nil, fmt.Errorf("%w: %d", ErrIntegerConversionOverflow, input[i])
			}
			params[i] = int32(input[i])
		case wasmtime.KindI64:
			params[i] = input[i]
		default:
			return nil, fmt.Errorf("%w: %v", ErrInvalidParamType, v.Kind())
		}
	}

	return params, nil
}
