// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"github.com/ava-labs/avalanchego/utils/units"
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
	DefaultMaxWasmStack         = 256 * units.MiB             // 256 MiB
	DefaultLimitMaxMemory       = uint32(18 * 64 * units.KiB) // 18 pages
	DefaultSIMD                 = false
	DefaultEnableReferenceTypes = false
	DefaultEnableBulkMemory     = false
	DefaultProfilingStrategy    = wasmtime.ProfilingStrategyNone
	DefaultMultiValue           = false
	DefaultCompileStrategy      = CompileWasm

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
		wasmConfig: DefaultWasmtimeConfig(),
	}
}

// Config is wrapper for wasmtime.Config
type Config struct {
	wasmConfig *wasmtime.Config
}

// Get returns the underlying wasmtime config.
func (c *Config) Get() *wasmtime.Config {
	return c.wasmConfig
}

// CacheConfigLoad enables compiled code caching for this `Config` using the
// settings specified in the configuration file `path`.
func (c *Config) CacheConfigLoad(path string) error {
	return c.wasmConfig.CacheConfigLoad(path)
}

// CacheConfigLoadDefault enables compiled code caching for this `Config` using
// the default settings configuration can be found.
func (c *Config) CacheConfigLoadDefault() error {
	return c.wasmConfig.CacheConfigLoadDefault()
}

// EnableCraneliftFlag enables a target-specific flag in Cranelift.
func (c *Config) EnableCraneliftFlag(flag string) {
	c.wasmConfig.EnableCraneliftFlag(flag)
}

// SetConsumFuel configures whether fuel is enabled.
func (c *Config) SetConsumeFuel(enabled bool) {
	c.wasmConfig.SetConsumeFuel(enabled)
}

// SetCraneliftDebugVerifier configures whether the cranelift debug verifier
// will be active when cranelift is used to compile wasm code.
func (c *Config) SetCraneliftDebugVerifier(enabled bool) {
	c.wasmConfig.SetCraneliftDebugVerifier(enabled)
}

// SetCraneliftFlag sets a target-specific flag in Cranelift to the specified value.
func (c *Config) SetCraneliftFlag(name string, value string) {
	c.wasmConfig.SetCraneliftFlag(name, value)
}

// SetCraneliftOptLevel configures the cranelift optimization level for generated code.
func (c *Config) SetCraneliftOptLevel(level wasmtime.OptLevel) {
	c.wasmConfig.SetCraneliftOptLevel(level)
}

// SetDebugInfo configures whether dwarf debug information for JIT code is enabled
func (c *Config) SetDebugInfo(enabled bool) {
	c.wasmConfig.SetDebugInfo(enabled)
}

// SetEpochInterruption enables epoch-based instrumentation of generated code to
// interrupt WebAssembly execution when the current engine epoch exceeds a
// defined threshold.
func (c *Config) SetEpochInterruption(enable bool) {
	c.wasmConfig.SetEpochInterruption(enable)
}

// SetMaxWasmStack configures the maximum stack size, in bytes, that JIT code can use.
func (c *Config) SetMaxWasmStack(size int) {
	c.wasmConfig.SetMaxWasmStack(size)
}

// SetProfiler configures what profiler strategy to use for generated code.
func (c *Config) SetProfiler(profiler wasmtime.ProfilingStrategy) {
	c.wasmConfig.SetProfiler(profiler)
}

// SetStrategy configures what compilation strategy is used to compile wasm code.
func (c *Config) SetStrategy(strategy wasmtime.Strategy) {
	c.wasmConfig.SetStrategy(strategy)
}

// SetTarget configures the target triple that this configuration will produce machine code for.
func (c *Config) SetTarget(target string) error {
	return c.wasmConfig.SetTarget(target)
}

// SetWasmBulkMemory configures whether the wasm bulk memory proposal is enabled.
func (c *Config) SetWasmBulkMemory(enabled bool) {
	c.wasmConfig.SetWasmBulkMemory(enabled)
}

// SetWasmMemory64 configures whether the wasm memory64 proposal is enabled.
func (c *Config) SetWasmMemory64(enabled bool) {
	c.wasmConfig.SetWasmMemory64(enabled)
}

// SetWasmMultiMemory configures whether the wasm multi memory proposal is enabled.
func (c *Config) SetWasmMultiMemory(enabled bool) {
	c.wasmConfig.SetWasmMultiMemory(enabled)
}

// SetWasmMultiValue configures whether the wasm multi value proposal is enabled.
func (c *Config) SetWasmMultiValue(enabled bool) {
	c.wasmConfig.SetWasmMultiValue(enabled)
}

// SetWasmReferenceTypes configures whether the wasm reference types proposal is enabled.
func (c *Config) SetWasmReferenceTypes(enabled bool) {
	c.wasmConfig.SetWasmReferenceTypes(enabled)
}

// SetWasmSIMD configures whether the wasm SIMD proposal is enabled.
func (c *Config) SetWasmSIMD(enabled bool) {
	c.wasmConfig.SetWasmSIMD(enabled)
}

// SetWasmThreads configures whether the wasm threads proposal is enabled.
func (c *Config) SetWasmThreads(enabled bool) {
	c.wasmConfig.SetWasmThreads(enabled)
}

// DefaultWasmtimeConfig returns a new wasmtime config with default settings.
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

// NewConfigBuilder returns a new engine configuration builder with default settings.
// All instances of ConfigBuilder should be created with this constructor.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		EnableBulkMemory:         DefaultEnableBulkMemory,
		EnableWasmMultiValue:     DefaultMultiValue,
		EnableWasmReferenceTypes: DefaultEnableReferenceTypes,
		EnableWasmSIMD:           DefaultSIMD,
		MaxWasmStack:             DefaultMaxWasmStack,
		ProfilingStrategy:        DefaultProfilingStrategy,
		EnableDefaultCache:       false,
	}
}

type ConfigBuilder struct {
	// Configures whether the WebAssembly bulk memory operations proposal will
	// be enabled for compilation.  This feature gates items such as the
	// memory.copy instruction, passive data/table segments, etc, being in a
	// module.
	// This is false by default.
	EnableBulkMemory bool `json:"enableBulkMemory,omitempty" yaml:"enable_bulk_memory,omitempty"`
	// Configures whether the WebAssembly multi-value proposal will be enabled for compilation.
	// This feature gates functions and blocks returning multiple values in a module, for example.
	// This is false by default.
	EnableWasmMultiValue bool `json:"enableWasmMultiValue,omitempty" yaml:"enable_wasm_multi_value,omitempty"`
	// Configures whether the WebAssembly reference types proposal will be
	// enabled for compilation.  This feature gates items such as the externref
	// and funcref types as well as allowing a module to define multiple tables.
	// Note that the reference types proposal depends on the bulk memory
	// proposal.
	// This is false by default.
	EnableWasmReferenceTypes bool `json:"enableWasmReferenceTypes,omitempty" yaml:"enable_wasm_reference_types,omitempty"`
	// Configures whether the WebAssembly SIMD proposal will be enabled for
	// compilation.  The WebAssembly SIMD proposal. This feature gates items
	// such as the v128 type and all of its operators being in a module. Note
	// that this does not enable the relaxed simd proposal.
	// This is false by default.
	EnableWasmSIMD bool `json:"enableWasmSIMD,omitempty" yaml:"enable_wasm_simd,omitempty"`
	// EnableDefaultCache enables compiled code caching for this `Config` using the default settings
	// configuration can be found.
	//
	// For more information about caching see
	// https://bytecodealliance.github.io/wasmtime/cli-cache.html
	// This is false by default.
	EnableDefaultCache bool `json:"enableDefaultCache,omitempty" yaml:"enable_default_cache,omitempty"`
	// SetMaxWasmStack configures the maximum stack size, in bytes, that JIT code can use.
	// The amount of stack space that wasm takes is always relative to the first invocation of wasm on the stack.
	// Recursive calls with host frames in the middle will all need to fit within this setting.
	// Note that this setting is not interpreted with 100% precision.
	// This is 256 MiB by default.
	MaxWasmStack int `json:"maxWasmStack,omitempty" yaml:"max_wasm_stack,omitempty"`
	// ProfilingStrategy decides what sort of profiling to enable, if any.
	// Default is `wasmtime.ProfilingStrategyNone`.
	ProfilingStrategy wasmtime.ProfilingStrategy
}

// WithMaxWasmStack defines the maximum amount of stack space available for
// executing WebAssembly code.
//
// Default is 256 MiB.
func (c *ConfigBuilder) WithMaxWasmStack(max int) *ConfigBuilder {
	c.MaxWasmStack = max
	return c
}

// WithMultiValue enables modules that can return multiple values.
// ref. https://github.com/webassembly/multi-value
//
// Default is false.
func (c *ConfigBuilder) WithMultiValue(enable bool) *ConfigBuilder {
	c.EnableWasmMultiValue = enable
	return c
}

// WithBulkMemory enables `memory.copy` instruction, tables and passive data.
// ref. https://github.com/WebAssembly/bulk-memory-operations
//
// Default is false.
func (c *ConfigBuilder) WithBulkMemory(enable bool) *ConfigBuilder {
	c.EnableBulkMemory = enable
	return c
}

// WithReferenceTypes Enables the `externref` and `funcref` types as well as
// allowing a module to define multiple tables.
// ref. https://github.com/webassembly/reference-types
//
// Note: depends on bulk memory being enabled.
//
// Default is false.
func (c *ConfigBuilder) WithReferenceTypes(enable bool) *ConfigBuilder {
	c.EnableWasmReferenceTypes = enable
	return c
}

// WithSIMD enables SIMD instructions including v128.
// ref. https://github.com/webassembly/simd
//
// Default is false.
func (c *ConfigBuilder) WithSIMD(enable bool) *ConfigBuilder {
	c.EnableWasmSIMD = enable
	return c
}

// WithProfilingStrategy defines the profiling strategy used for defining the
// default profiler.
//
// Default is `wasmtime.ProfilingStrategyNone`.
func (c *ConfigBuilder) WithProfilingStrategy(strategy wasmtime.ProfilingStrategy) *ConfigBuilder {
	c.ProfilingStrategy = strategy
	return c
}

// WithDefaultCache enables the default caching strategy.
//
// Default is false.
func (c *ConfigBuilder) WithDefaultCache(enabled bool) *ConfigBuilder {
	c.EnableDefaultCache = enabled
	return c
}

func (c *ConfigBuilder) Build() (*Config, error) {
	cfg := NewConfig()
	cfg.SetWasmBulkMemory(c.EnableBulkMemory)
	cfg.SetWasmMultiValue(c.EnableWasmMultiValue)
	cfg.SetWasmReferenceTypes(c.EnableWasmReferenceTypes)
	cfg.SetWasmSIMD(c.EnableWasmSIMD)
	cfg.SetMaxWasmStack(c.MaxWasmStack)
	cfg.SetProfiler(c.ProfilingStrategy)
	if c.EnableDefaultCache {
		if err := cfg.CacheConfigLoadDefault(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
