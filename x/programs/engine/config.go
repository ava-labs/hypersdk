// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
)

type CompileStrategy uint8

const (
	// CompileWasm will compile the wasm module before instantiating it.
	CompileWasm CompileStrategy = iota
	// PrecompiledWasm accepts a precompiled wasm module serialized by an Engine.
	PrecompiledWasm
)

var (
	DefaultMaxWasmStack         = 256 * 1024 * 1024     // 256 MiB
	DefaultLimitMaxMemory       = int64(18 * 64 * 1024) // 18 pages
	DefaultSIMD                 = false
	DefaultEnableReferenceTypes = false
	DefaultEnableBulkMemory     = false
	DefaultProfilingStrategy    = wasmtime.ProfilingStrategyNone
	DefaultMultiValue           = false
	DefaultCompileStrategy	  = CompileWasm

	defaultWasmThreads                  = false
	defaultFuelMetering                 = true
	defaultWasmMultiMemory              = false
	defaultWasmMemory64                 = false
	defaultCompilerStrategy             = wasmtime.StrategyCranelift
	defaultEpochInterruption            = true
	defaultNaNCanonicalization          = "true"
	defaultCraneliftOptLevel            = wasmtime.OptLevelSpeed
	defaultEnableCraneliftDebugVerifier = false
	defaultEnableDebugInfo              = false
)

// NewConfig creates a new engine config with default settings
func NewConfig() *Config {
	return &Config{
		inner: DefaultWasmtimeConfig(),
	}
}

// Config is wrapper for wasmtime.Config
type Config struct {
	inner *wasmtime.Config
}

// CacheConfigLoad enables compiled code caching for this `Config` using the
// settings specified in the configuration file `path`.
func (c *Config) CacheConfigLoad(path string) error {
	return c.inner.CacheConfigLoad(path)
}

// CacheConfigLoadDefault enables compiled code caching for this `Config` using
// the default settings configuration can be found.
func (c *Config) CacheConfigLoadDefault() error {
	return c.inner.CacheConfigLoadDefault()
}

// EnableCraneliftFlag enables a target-specific flag in Cranelift.
func (c *Config) EnableCraneliftFlag(flag string) {
	c.inner.EnableCraneliftFlag(flag)
}

// SetConsumFuel configures whether fuel is enabled.
func (c *Config) SetConsumeFuel(enabled bool) {
	c.inner.SetConsumeFuel(enabled)
}

// SetCraneliftDebugVerifier configures whether the cranelift debug verifier
// will be active when cranelift is used to compile wasm code.
func (c *Config) SetCraneliftDebugVerifier(enabled bool) {
	c.inner.SetCraneliftDebugVerifier(enabled)
}

// SetCraneliftFlag sets a target-specific flag in Cranelift to the specified value.
func (c *Config) SetCraneliftFlag(name string, value string) {
	c.inner.SetCraneliftFlag(name, value)
}

// SetCraneliftOptLevel configures the cranelift optimization level for generated code.
func (c *Config) SetCraneliftOptLevel(level wasmtime.OptLevel) {
	c.inner.SetCraneliftOptLevel(level)
}

// SetDebugInfo configures whether dwarf debug information for JIT code is enabled
func (c *Config) SetDebugInfo(enabled bool) {
	c.inner.SetDebugInfo(enabled)
}

// SetEpochInterruption enables epoch-based instrumentation of generated code to
// interrupt WebAssembly execution when the current engine epoch exceeds a
// defined threshold.
func (c *Config) SetEpochInterruption(enable bool) {
	c.inner.SetEpochInterruption(enable)
}

// SetMaxWasmStack configures the maximum stack size, in bytes, that JIT code can use.
func (c *Config) SetMaxWasmStack(size int) {
	c.inner.SetMaxWasmStack(size)
}

// SetProfiler configures what profiler strategy to use for generated code.
func (c *Config) SetProfiler(profiler wasmtime.ProfilingStrategy) {
	c.inner.SetProfiler(profiler)
}

// SetStrategy configures what compilation strategy is used to compile wasm code.
func (c *Config) SetStrategy(strat wasmtime.Strategy) {
	c.inner.SetStrategy(strat)
}

// SetTarget configures the target triple that this configuration will produce machine code for.
func (c *Config) SetTarget(target string) error {
	return c.inner.SetTarget(target)
}

// SetWasmBulkMemory configures whether the wasm bulk memory proposal is enabled.
func (c *Config) SetWasmBulkMemory(enabled bool) {
	c.inner.SetWasmBulkMemory(enabled)
}

// SetWasmMemory64 configures whether the wasm memory64 proposal is enabled.
func (c *Config) SetWasmMemory64(enabled bool) {
	c.inner.SetWasmMemory64(enabled)
}

// SetWasmMultiMemory configures whether the wasm multi memory proposal is enabled.
func (c *Config) SetWasmMultiMemory(enabled bool) {
	c.inner.SetWasmMultiMemory(enabled)
}

// SetWasmMultiValue configures whether the wasm multi value proposal is enabled.
func (c *Config) SetWasmMultiValue(enabled bool) {
	c.inner.SetWasmMultiValue(enabled)
}

// SetWasmReferenceTypes configures whether the wasm reference types proposal is enabled.
func (c *Config) SetWasmReferenceTypes(enabled bool) {
	c.inner.SetWasmReferenceTypes(enabled)
}

// SetWasmSIMD configures whether the wasm SIMD proposal is enabled.
func (c *Config) SetWasmSIMD(enabled bool) {
	c.inner.SetWasmSIMD(enabled)
}

// SetWasmThreads configures whether the wasm threads proposal is enabled.
func (c *Config) SetWasmThreads(enabled bool) {
	c.inner.SetWasmThreads(enabled)
}

func DefaultWasmtimeConfig() *wasmtime.Config {
	cfg := wasmtime.NewConfig()

	// non configurable defaults
	cfg.SetCraneliftOptLevel(defaultCraneliftOptLevel)
	cfg.SetConsumeFuel(defaultFuelMetering)
	cfg.SetWasmThreads(defaultWasmThreads)
	cfg.SetWasmMultiMemory(defaultWasmMultiMemory)
	cfg.SetWasmMemory64(defaultWasmMemory64)
	cfg.SetStrategy(defaultCompilerStrategy)
	cfg.SetEpochInterruption(defaultEpochInterruption)
	cfg.SetCraneliftFlag("enable_nan_canonicalization", defaultNaNCanonicalization)

	// TODO: expose these knobs for developers
	cfg.SetCraneliftDebugVerifier(defaultEnableCraneliftDebugVerifier)
	cfg.SetDebugInfo(defaultEnableDebugInfo)
	return cfg
}
