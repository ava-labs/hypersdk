// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/codec"
)

type WasmRuntime struct {
	log           logging.Logger
	engine        *wasmtime.Engine
	hostImports   *Imports
	cfg           *Config
	programs      map[codec.Address]*Program
	programLoader ProgramLoader
}

type ProgramLoader interface {
	GetProgramBytes(ctx context.Context, program codec.Address) ([]byte, error)
}

func NewRuntime(
	cfg *Config,
	log logging.Logger,
	loader ProgramLoader,
) *WasmRuntime {
	runtime := &WasmRuntime{
		log:           log,
		cfg:           cfg,
		engine:        wasmtime.NewEngineWithConfig(cfg.wasmConfig),
		hostImports:   NewImports(),
		programs:      map[codec.Address]*Program{},
		programLoader: loader,
	}

	runtime.AddImportModule(NewLogModule())
	runtime.AddImportModule(NewStateAccessModule())
	runtime.AddImportModule(NewProgramModule(runtime))

	return runtime
}

func (r *WasmRuntime) AddImportModule(mod *ImportModule) {
	r.hostImports.AddModule(mod)
}

func (r *WasmRuntime) AddProgram(program codec.Address, bytes []byte) error {
	programModule, err := newProgram(r.engine, bytes)
	if err != nil {
		return err
	}
	r.programs[program] = programModule
	return nil
}

func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	program, ok := r.programs[callInfo.Program]
	if !ok {
		bytes, err := r.programLoader.GetProgramBytes(ctx, callInfo.Program)
		if err != nil {
			return nil, err
		}
		program, err = newProgram(r.engine, bytes)
		if err != nil {
			return nil, err
		}
		r.programs[callInfo.Program] = program
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
