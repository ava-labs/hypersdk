// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

// Engine is a wrapper around a wasmtime.Engine and manages the lifecycle of a
// programs execution. It is expected that a single engine can have multiple
// stores. For example in the case of program to program calls.
type Engine struct {
	inner *wasmtime.Engine
}

// New creates a new Wasm engine.
func New(cfg *Config) *Engine {
	return &Engine{
		inner: wasmtime.NewEngineWithConfig(cfg.inner),
	}
}

func (e *Engine) IncrementEpoch() {
	e.inner.IncrementEpoch()
}

func (e *Engine) PreCompileModule(bytes []byte) (*wasmtime.Module, error) {
	// Note: that to deserialize successfully the bytes provided must have been
	// produced with an `Engine` that has the same compilation options as the
	// provided engine, and from the same version of this library.
	//
	// A precompile is not something we would store on chain.
	// Instead we would prefetch programs and precompile them.
	return wasmtime.NewModuleDeserialize(e.inner, bytes)
}

func (e *Engine) CompileModule(bytes []byte) (*wasmtime.Module, error) {
	return wasmtime.NewModule(e.inner, bytes)
}

// PreCompileWasm returns a precompiled wasm module.
//
// Note: these bytes can be deserialized by an `Engine` that has the same version.
// For that reason precompiled wasm modules should not be stored on chain.
func PreCompileWasmBytes(programBytes []byte, engineCfg *Config, limitMaxMemory int64) ([]byte, error) {
	storeCfg := NewStoreConfig()
	storeCfg.SetLimitMaxMemory(limitMaxMemory)
	store := NewStore(New(engineCfg), storeCfg)
	module, err := wasmtime.NewModule(store.Engine(), programBytes)
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
		return nil, fmt.Errorf("unknown compile strategy")
	}
}
