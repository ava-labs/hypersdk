// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

type WasmRuntime struct {
	log           logging.Logger
	engine        *wasmtime.Engine
	hostImports   *Imports
	cfg           *Config
	programs      map[codec.Address]*Program
	programLoader ProgramLoader
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
) *WasmRuntime {
	runtime := &WasmRuntime{
		log:           log,
		cfg:           cfg,
		engine:        wasmtime.NewEngineWithConfig(cfg.wasmConfig),
		hostImports:   NewImports(),
		programs:      map[codec.Address]*Program{},
		programLoader: programLoader,
	}

	runtime.AddImportModule(NewLogModule())
	runtime.AddImportModule(NewStateAccessModule())
	runtime.AddImportModule(NewProgramModule(runtime))

	return runtime
}

func (r *WasmRuntime) AddImportModule(mod *ImportModule) {
	r.hostImports.AddModule(mod)
}

func (r *WasmRuntime) AddProgram(program codec.Address, bytes []byte) (*Program, error) {
	programModule, err := newProgram(r.engine, bytes)
	if err != nil {
		return nil, err
	}
	r.programs[program] = programModule
	return programModule, nil
}

func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	program, ok := r.programs[callInfo.Program]
	if !ok {
		bytes, err := r.programLoader.GetProgramBytes(ctx, callInfo.Program)
		if err != nil {
			return nil, err
		}
		program, err = r.AddProgram(callInfo.Program, bytes)
		if err != nil {
			return nil, err
		}
	}
	inst, err := r.getInstance(callInfo, program, r.hostImports)
	if err != nil {
		return nil, err
	}
	callInfo.inst = inst
	return inst.call(ctx, callInfo)
}

func (r *WasmRuntime) getInstance(callInfo *CallInfo, program *Program, imports *Imports) (*ProgramInstance, error) {
	linker, err := imports.createLinker(r.engine, callInfo)
	if err != nil {
		return nil, err
	}
	store := wasmtime.NewStore(r.engine)
	store.SetEpochDeadline(1)
	inst, err := linker.Instantiate(store, program.module)
	if err != nil {
		return nil, err
	}
	return &ProgramInstance{inst: inst, store: store}, nil
}
