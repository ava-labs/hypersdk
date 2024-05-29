// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

// #include "shims.h"
import "C"
import (
	"context"
	"reflect"
	"unsafe"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
)

type WasmRuntime struct {
	log           logging.Logger
	engine        *wasmtime.Engine
	hostImports   *Imports
	cfg           *Config
	programs      map[ids.ID]*Program
	callerInfo    map[*C.wasmtime_context_t]*CallInfo
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
		programs:      map[ids.ID]*Program{},
		programLoader: loader,
		callerInfo:    map[*C.wasmtime_context_t]*CallInfo{},
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
	r.programs[programID] = programModule
	return nil
}

func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	program, ok := r.programs[callInfo.ProgramID]
	if !ok {
		bytes, err := r.programLoader.GetProgramBytes(ctx, callInfo.ProgramID)
		if err != nil {
			return nil, err
		}
		program, err = newProgram(r.engine, callInfo.ProgramID, bytes)
		if err != nil {
			return nil, err
		}
		r.programs[callInfo.ProgramID] = program
	}
	inst, err := r.getInstance(program)
	if err != nil {
		return nil, err
	}
	callInfo.inst = inst
	key := convert(inst.store)
	r.callerInfo[key] = callInfo
	defer delete(r.callerInfo, key)
	return inst.call(ctx, callInfo)
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

func convert(storelike wasmtime.Storelike) *C.wasmtime_context_t {
	return (*C.wasmtime_context_t)(unsafe.Pointer(reflect.ValueOf(storelike.Context()).Pointer()))
}
