// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

type Engine struct {
	inner *wasmtime.Engine
	cfg   *Config
}

// New creates a new Wasm engine
func New(cfg *Config) (*Engine, error) {
	wcfg, err := cfg.Engine()
	if err != nil {
		return nil, err
	}
	return &Engine{
		cfg:   cfg,
		inner: wasmtime.NewEngineWithConfig(wcfg),
	}, nil
}

func (e *Engine) CompileModule(bytes []byte) (*wasmtime.Module, error) {
	switch e.cfg.CompileStrategy {
	case PrecompiledWasm:
		// Note: that to deserialize successfully the bytes provided must have been
		// produced with an `Engine` that has the same compilation options as the
		// provided engine, and from the same version of this library.
		//
		// A precompile is not something we would store on chain.
		// Instead we would prefetch programs and precompile them.
		return wasmtime.NewModuleDeserialize(e.inner, bytes)

	case CompileWasm:
		return wasmtime.NewModule(e.inner, bytes)
	default:
		return nil, fmt.Errorf("unsupported compile strategy: %v", e.cfg.CompileStrategy)
	}
}

// PreCompileWasm returns a precompiled wasm module.
//
// Note: these bytes can be deserialized by an `Engine` that has the same version.
// For that reason precompiled wasm modules should not be stored on chain.
func PreCompileWasmBytes(programBytes []byte, cfg *Config) ([]byte, error) {
	engine, err := New(cfg)
	if err != nil {
		return nil, err
	}
	store, err := NewStore(engine)
	if err != nil {
		return nil, err
	}
	module, err := wasmtime.NewModule(store.Engine(), programBytes)
	if err != nil {
		return nil, err
	}

	return module.Serialize()
}
