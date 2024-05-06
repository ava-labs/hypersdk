// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"context"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/state"
	"github.com/bytecodealliance/wasmtime-go/v14"
)

type WasmRuntime struct {
	log         logging.Logger
	engine      *wasmtime.Engine
	hostImports *Imports
	cfg         *Config
	programs    map[ProgramID]*Program
	chainState  state.Mutable
}

func NewRuntime(
	log logging.Logger,
	chainState state.Mutable,
	cfg *Config,
) *WasmRuntime {
	runtime := &WasmRuntime{
		log:         log,
		engine:      wasmtime.NewEngineWithConfig(cfg.wasmConfig),
		chainState:  chainState,
		cfg:         cfg,
		hostImports: NewImports(),
		programs:    map[ProgramID]*Program{},
	}
	runtime.hostImports.AddModule(NewStateAccessModule())
	runtime.hostImports.AddModule(NewCallProgramModule(runtime))
	return runtime
}

func (r *WasmRuntime) AddImports(mod *ImportModule) {
	r.hostImports.AddModule(mod)
}

func (r *WasmRuntime) AddProgram(programID ProgramID, bytes []byte) error {
	programModule, err := newProgram(r.engine, programID, bytes)
	if err != nil {
		return err
	}
	r.programs[programID] = programModule
	return nil
}
func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	program, ok := r.programs[callInfo.Program]
	if !ok {
		// load program into memory
	}
	inst, err := r.getInstance(callInfo, program, r.hostImports)
	if err != nil {
		return nil, err
	}
	callInfo.FunctionName = callInfo.FunctionName + "_guest"
	return inst.call(ctx, callInfo)
}

func (r *WasmRuntime) getInstance(callInfo *CallInfo, program *Program, imports *Imports) (*ProgramInstance, error) {
	linker, err := imports.createLinker(r.engine, callInfo)
	store := wasmtime.NewStore(r.engine)
	store.SetEpochDeadline(1)
	inst, err := linker.Instantiate(store, program.module)
	if err != nil {
		return nil, err
	}
	return &ProgramInstance{Program: program, inst: inst, store: store}, nil
}
