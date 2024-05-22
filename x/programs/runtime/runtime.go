// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"
	"os"

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
	programLoader ProgramLoader
}

type ProgramLoader interface {
	GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error)
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
		programs:      map[ids.ID]*Program{},
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

func (r *WasmRuntime) AddProgram(programID ids.ID, bytes []byte) error {
	programModule, err := newProgram(r.engine, programID, bytes)
	if err != nil {
		return err
	}
	r.programs[programID] = programModule
	return nil
}

func (r *WasmRuntime) CallProgram(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	fmt.Fprintln(os.Stderr, "CallProgram")
	program, ok := r.programs[callInfo.ProgramID]
	if !ok {
		bytes, err := r.programLoader.GetProgramBytes(ctx, callInfo.ProgramID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "err1")
			return nil, err
		}
		program, err = newProgram(r.engine, callInfo.ProgramID, bytes)
		if err != nil {
			fmt.Fprintln(os.Stderr, "err2")
			return nil, err
		}
		r.programs[callInfo.ProgramID] = program
	}
	inst, err := r.getInstance(callInfo, program, r.hostImports)
	if err != nil {
		fmt.Fprintln(os.Stderr, "err3")
		return nil, err
	}
	callInfo.inst = inst
	callInfo.FunctionName += "_guest"
	fmt.Fprintln(os.Stderr, "call")
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
