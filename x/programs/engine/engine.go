// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"errors"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

// Engine is a wrapper around a wasmtime.Engine and manages the lifecycle of a
// programs execution. It is expected that a single engine can have multiple
// stores. For example in the case of program to program calls.
type Engine struct {
	wasmEngine *wasmtime.Engine
}

// New creates a new Wasm engine.
func New(cfg *Config) *Engine {
	return &Engine{
		wasmEngine: wasmtime.NewEngineWithConfig(cfg.Get()),
	}
}

// Stop will increase the current epoch number by 1 within the current
// engine which will cause any connected stores to be interrupted.
func (e *Engine) Stop() {
	e.wasmEngine.IncrementEpoch()
}

// PreCompileModule will deserialize a precompiled module.
func (e *Engine) PreCompileModule(bytes []byte) (*wasmtime.Module, error) {
	// Note: that to deserialize successfully the bytes provided must have been
	// produced with an `Engine` that has the same compilation options as the
	// provided engine, and from the same version of this library.
	//
	// A precompile is not something we would store on chain.
	// Instead we would prefetch programs and precompile them.
	return wasmtime.NewModuleDeserialize(e.wasmEngine, bytes)
}

// CompileModule will compile a module.
func (e *Engine) CompileModule(bytes []byte) (*wasmtime.Module, error) {
	return wasmtime.NewModule(e.wasmEngine, bytes)
}

// PreCompileWasm returns a precompiled wasm module.
//
// Note: these bytes can be deserialized by an `Engine` that has the same version.
// For that reason precompiled wasm modules should not be stored on chain.
func PreCompileWasmBytes(engine *Engine, programBytes []byte, limitMaxMemory uint32) ([]byte, error) {
	store := NewStore(engine, NewStoreConfig().SetLimitMaxMemory(limitMaxMemory))
	module, err := wasmtime.NewModule(store.GetEngine(), programBytes)
	if err != nil {
		return nil, err
	}

	return module.Serialize()
}

// NewModule creates a new wasmtime module and handles the Wasm bytes based on compile strategy.
func NewModule(engine *Engine, bytes []byte, strategy CompileStrategy) (*wasmtime.Module, error) {
	switch strategy {
	case CompileWasm:
		return engine.CompileModule(bytes)
	case PrecompiledWasm:
		return engine.PreCompileModule(bytes)
	default:
		return nil, errors.New("unknown compile strategy")
	}
}
