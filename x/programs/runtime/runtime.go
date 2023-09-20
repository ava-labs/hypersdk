// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v12"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
)

var _ Runtime = &runtime{}

// New returns a new wasm runtime.
func New(log logging.Logger, cfg *Config, imports Imports) *runtime {
	if imports == nil {
		imports = make(Imports)
	}
	return &runtime{
		imports: imports,
		log:     log,
		cfg:     cfg,
	}
}

type runtime struct {
	cfg   *Config
	inst  *wasmtime.Instance
	store *wasmtime.Store
	mod   *wasmtime.Module
	meter Meter

	// imports available to the guest
	imports Imports

	log logging.Logger
}

func (r *runtime) Initialize(_ context.Context, programBytes ProgramBytes) (err error) {
	r.store = wasmtime.NewStore(wasmtime.NewEngineWithConfig(r.cfg.engine))
	r.store.Limiter(
		r.cfg.limitMaxMemory,
		r.cfg.limitMaxTableElements,
		r.cfg.limitMaxInstances,
		r.cfg.limitMaxTables,
		r.cfg.limitMaxMemories,
	)

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

	// TODO: remove wasi host functions.
	// initialize wasi
	wcfg := wasmtime.NewWasiConfig()
	wcfg.InheritStderr()
	wcfg.InheritStdout()
	r.store.SetWasi(wcfg)

	link := Link{wasmtime.NewLinker(r.store.Engine)}
	err = link.DefineWasi()
	if err != nil {
		return err
	}

	r.meter = NewMeter(r.store)
	err = r.meter.AddUnits(r.cfg.meterMaxUnits)
	if err != nil {
		return err
	}

	imports := getRegisteredImportModules(r.mod.Imports())
	// register host functions exposed to the guest (imports)
	for _, imp := range imports {
		// registered separately by linker
		if imp == wasiPreview1ModName {
			continue
		}
		mod, ok := r.imports[imp]
		if !ok {
			return fmt.Errorf("%w: %s", ErrMissingImportModule, imp)
		}
		r.log.Debug("registering host functions for module",
			zap.String("name", imp),
		)
		err = mod.Register(link, r.meter)
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
	imports := make([]string, len(importTypes))
	for _, t := range importTypes {
		mod := t.Module()
		if _, ok := u[mod]; ok {
			continue
		}
		u[mod] = struct{}{}
		imports = append(imports, mod)
	}
	return imports
}

func (r *runtime) Call(_ context.Context, name string, params ...interface{}) ([]uint64, error) {
	var api *wasmtime.Func

	switch name {
	case AllocFnName, DeallocFnName, MemoryFnName:
		api = r.inst.GetFunc(r.store, name)
	default:
		api = r.inst.GetFunc(r.store, name+guestSuffix)
	}
	if api == nil {
		return nil, ErrMissingExportedFunction
	}

	result, err := api.Call(r.store, params...)
	if err != nil {
		return nil, fmt.Errorf("export function call failed %s: %w", name, err)
	}

	switch v := result.(type) {
	case int32:
		value := uint64(result.(int32))
		return []uint64{value}, nil
	case int64:
		value := uint64(result.(int64))
		return []uint64{value}, nil
	default:
		return nil, fmt.Errorf("invalid result type: %v", v)
	}
}

func (r *runtime) Memory() Memory {
	return NewMemory(newExportClient(r.inst, r.store))
}

func (r *runtime) Meter() Meter {
	return r.meter
}

func (r *runtime) Stop(_ context.Context) error {
	// TODO: add test
	r.store.SetEpochDeadline(0)
	return nil
}
