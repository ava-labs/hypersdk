// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

func NewStore(cfg *Config) (*Store, error) {
	engineConfig, err := cfg.Engine()
	if err != nil {
		return nil, err
	}

	inner := wasmtime.NewStore(wasmtime.NewEngineWithConfig(engineConfig))
	inner.Limiter(
		cfg.limitMaxMemory,
		cfg.limitMaxTableElements,
		cfg.limitMaxInstances,
		cfg.limitMaxTables,
		cfg.limitMaxMemories,
	)

	return &Store{inner: inner}, nil
}

type Store struct {
	inner           *wasmtime.Store
	compileStrategy CompileStrategy
}

func (s *Store) SetEpochDeadline(epochDeadline uint64) {
	s.inner.SetEpochDeadline(epochDeadline)
}

func (s *Store) Engine() *wasmtime.Engine {
	return s.inner.Engine
}

func (s *Store) FuelConsumed() (uint64, bool) {
	return s.inner.FuelConsumed()
}

func (s *Store) ConsumeFuel(units uint64) (uint64, error) {
	return s.inner.ConsumeFuel(units)
}

func (s *Store) AddFuel(units uint64) error {
	return s.inner.AddFuel(units)
}

func (s *Store) SetWasi(cfg *wasmtime.WasiConfig) {
	s.inner.SetWasi(cfg)
}

func (s *Store) Inner() wasmtime.Storelike {
	return s.inner
}

func (s *Store) CompileModule(bytes []byte) (*wasmtime.Module, error) {
	switch s.compileStrategy {
	case PrecompiledWasm:
		// Note: that to deserialize successfully the bytes provided must have been
		// produced with an `Engine` that has the same compilation options as the
		// provided engine, and from the same version of this library.
		//
		// A precompile is not something we would store on chain.
		// Instead we would prefetch programs and precompile them.
		return wasmtime.NewModuleDeserialize(s.Engine(), bytes)

	case CompileWasm:
		return wasmtime.NewModule(s.Engine(), bytes)
	default:
		return nil, fmt.Errorf("unsupported compile strategy: %v", s.compileStrategy)
	}
}

// PreCompileWasm returns a precompiled wasm module.
//
// Note: these bytes can be deserialized by an `Engine` that has the same version.
// For that reason precompiled wasm modules should not be stored on chain.
func PreCompileWasmBytes(programBytes []byte, cfg *Config) ([]byte, error) {
	engineConfig, err := cfg.Engine()
	if err != nil {
		return nil, err
	}

	store := wasmtime.NewStore(wasmtime.NewEngineWithConfig(engineConfig))
	store.Limiter(
		cfg.limitMaxMemory,
		cfg.limitMaxTableElements,
		cfg.limitMaxInstances,
		cfg.limitMaxTables,
		cfg.limitMaxMemories,
	)

	module, err := wasmtime.NewModule(store.Engine, programBytes)
	if err != nil {
		return nil, err
	}

	return module.Serialize()
}
