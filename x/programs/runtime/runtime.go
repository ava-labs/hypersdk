// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"reflect"
	"slices"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime/collections"
)

type WasmRuntime struct {
	log           logging.Logger
	engine        *wasmtime.Engine
	hostImports   *Imports
	cfg           *Config
	callerInfo    map[uintptr]*collections.FixedSizeStack[*CallInfo]
	programs      map[codec.Address]*ProgramInstance
	programLoader ProgramLoader
	linker        *wasmtime.Linker
}

type StateLoader interface {
	GetProgramState(address codec.Address) state.Mutable
}

type ProgramLoader interface {
	GetProgramBytes(ctx context.Context, address codec.Address) ([]byte, error)
}

func NewRuntime(
	cfg *Config,
	log logging.Logger,
	programLoader ProgramLoader,
) (*WasmRuntime, error) {
	runtime := &WasmRuntime{
		log:           log,
		cfg:           cfg,
		engine:        wasmtime.NewEngineWithConfig(cfg.wasmConfig),
		hostImports:   NewImports(),
		callerInfo:    map[uintptr]*collections.FixedSizeStack[*CallInfo]{},
		programs:      map[codec.Address]*ProgramInstance{},
		programLoader: programLoader,
	}

	runtime.AddImportModule(NewLogModule())
	runtime.AddImportModule(NewStateAccessModule())
	runtime.AddImportModule(NewProgramModule(runtime))
	linker, err := runtime.hostImports.createLinker(runtime)
	runtime.linker = linker
	return runtime, err
}

func (r *WasmRuntime) AddImportModule(mod *ImportModule) {
	r.hostImports.AddModule(mod)
}

func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	program, ok := r.programs[callInfo.Program]
	if !ok {
		bytes, err := r.programLoader.GetProgramBytes(ctx, callInfo.Program)
		if err != nil {
			return nil, err
		}
		programMod, err := newProgram(r.engine, bytes)
		if err != nil {
			return nil, err
		}
		program, err = r.getInstance(programMod)
		if err != nil {
			return nil, err
		}
		r.programs[callInfo.Program] = program
		key := toMapKey(program.store)
		r.callerInfo[key] = collections.NewFixedSizeStack[*CallInfo](100)
	}
	callInfo.inst = program
	if err := r.setCallInfo(program.store, callInfo); err != nil {
		return nil, err
	}
	defer r.deleteCallInfo(program.store)
	return program.call(ctx, callInfo)
}

func (r *WasmRuntime) getInstance(program *Program) (*ProgramInstance, error) {
	store := wasmtime.NewStore(r.engine)
	store.SetEpochDeadline(1)
	inst, err := r.linker.Instantiate(store, program.module)
	if err != nil {
		return nil, err
	}

	return &ProgramInstance{inst: inst, store: store, resetStore: slices.Clone(inst.GetExport(store, MemoryName).Memory().UnsafeData(store))}, nil
}

func toMapKey(storelike wasmtime.Storelike) uintptr {
	return reflect.ValueOf(storelike.Context()).Pointer()
}

func (r *WasmRuntime) setCallInfo(storelike wasmtime.Storelike, info *CallInfo) error {
	return r.callerInfo[toMapKey(storelike)].Push(info)
}

func (r *WasmRuntime) getCallInfo(storelike wasmtime.Storelike) *CallInfo {
	return r.callerInfo[toMapKey(storelike)].Peek()
}

func (r *WasmRuntime) deleteCallInfo(storelike wasmtime.Storelike) {
	r.callerInfo[toMapKey(storelike)].Pop()
}
