// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import "C"
import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/runtime/collections"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"reflect"
)

type WasmRuntime struct {
	log           logging.Logger
	engine        *wasmtime.Engine
	hostImports   *Imports
	cfg           *Config
	programs      map[ids.ID]*ProgramInstance
	callerInfo    map[uintptr]*collections.Stack[*CallInfo]
	programLoader ProgramLoader
	linker        *wasmtime.Linker
}

type ProgramLoader interface {
	GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error)
}

func NewRuntime(
	cfg *Config,
	log logging.Logger,
	loader ProgramLoader,
) (*WasmRuntime, error) {
	runtime := &WasmRuntime{
		log:           log,
		cfg:           cfg,
		engine:        wasmtime.NewEngineWithConfig(cfg.wasmConfig),
		hostImports:   NewImports(),
		programs:      map[ids.ID]*ProgramInstance{},
		programLoader: loader,
		callerInfo:    map[uintptr]*collections.Stack[*CallInfo]{},
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

func (r *WasmRuntime) AddProgram(programID ids.ID, bytes []byte) error {
	programModule, err := newProgram(r.engine, programID, bytes)
	if err != nil {
		return err
	}
	program, err := r.getInstance(programModule)
	if err != nil {
		return err
	}
	r.programs[programID] = program
	return nil
}

func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	program, ok := r.programs[callInfo.ProgramID]
	if !ok {
		bytes, err := r.programLoader.GetProgramBytes(ctx, callInfo.ProgramID)
		if err != nil {
			return nil, err
		}
		programMod, err := newProgram(r.engine, callInfo.ProgramID, bytes)
		if err != nil {
			return nil, err
		}
		program, err = r.getInstance(programMod)
		if err != nil {
			return nil, err
		}
		r.programs[callInfo.ProgramID] = program
		key := toMapKey(program.store)
		callstack := make(collections.Stack[*CallInfo], 0, 2)
		r.callerInfo[key] = &callstack
	}
	callInfo.inst = program
	r.setCallInfo(program.store, callInfo)
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
	return &ProgramInstance{inst: inst, store: store}, nil
}

func toMapKey(storelike wasmtime.Storelike) uintptr {
	return reflect.ValueOf(storelike.Context()).Pointer()
}

func (r *WasmRuntime) setCallInfo(storelike wasmtime.Storelike, info *CallInfo) {
	r.callerInfo[toMapKey(storelike)].Push(info)
}

func (r *WasmRuntime) getCallInfo(storelike wasmtime.Storelike) *CallInfo {
	return r.callerInfo[toMapKey(storelike)].Peek()
}

func (r *WasmRuntime) deleteCallInfo(storelike wasmtime.Storelike) {
	r.callerInfo[toMapKey(storelike)].Pop()
}
