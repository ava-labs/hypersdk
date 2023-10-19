// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"math"

	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/bytecodealliance/wasmtime-go/v13"
)

const (
	defaultMaxWasmStack                 = 256 * 1024 * 1024 // 256 MiB
	defaultWasmThreads                  = false
	defaultFuelMetering                 = true
	defaultWasmMultiMemory              = false
	defaultWasmMemory64                 = false
	defaultLimitMaxMemory               = 18 * 64 * 1024 // 18 pages
	defaultSIMD                         = false
	defaultCompilerStrategy             = wasmtime.StrategyCranelift
	defaultEpochInterruption            = true
	defaultNaNCanonicalization          = "true"
	defaultCraneliftOptLevel            = wasmtime.OptLevelSpeed
	defaultEnableReferenceTypes         = false
	defaultEnableBulkMemory             = false
	defaultProfilingStrategy            = wasmtime.ProfilingStrategyNone
	defaultMultiValue                   = false
	defaultEnableCraneliftDebugVerifier = false
	defaultEnableDebugInfo              = false
	defaultCompileStrategy              = CompileWasm

	defaultLimitMaxTableElements = 4096
	defaultLimitMaxTables        = 1
	defaultLimitMaxInstances     = 32
	defaultLimitMaxMemories      = 1
)

// NewConfigBuilder returns a new runtime configuration builder with default settings.
// All instances of ConfigBuilder should be created with this constructor.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		EnableBulkMemory:         defaultEnableBulkMemory,
		EnableWasmMultiValue:     defaultMultiValue,
		EnableWasmReferenceTypes: defaultEnableReferenceTypes,
		EnableWasmSIMD:           defaultSIMD,
		MaxWasmStack:             defaultMaxWasmStack,
		LimitMaxMemory:           defaultLimitMaxMemory,
		ProfilingStrategy:        defaultProfilingStrategy,
		CompileStrategy:          defaultCompileStrategy,
		EnableDefaultCache:       false,
		EnableTestingOnlyMode:    false,
	}
}

type ConfigBuilder struct {
	// Configures whether the WebAssembly bulk memory operations proposal will
	// be enabled for compilation.  This feature gates items such as the
	// memory.copy instruction, passive data/table segments, etc, being in a
	// module.
	// This is false by default.
	EnableBulkMemory bool `yaml:"enable_bulk_memory,omitempty" json:"enableBulkMemory,omitempty"`
	// Configures whether the WebAssembly multi-value proposal will be enabled for compilation.
	// This feature gates functions and blocks returning multiple values in a module, for example.
	// This is false by default.
	EnableWasmMultiValue bool `yaml:"enable_wasm_multi_value,omitempty" json:"enableWasmMultiValue,omitempty"`
	// Configures whether the WebAssembly reference types proposal will be
	// enabled for compilation.  This feature gates items such as the externref
	// and funcref types as well as allowing a module to define multiple tables.
	// Note that the reference types proposal depends on the bulk memory
	// proposal.
	// This is false by default.
	EnableWasmReferenceTypes bool `yaml:"enable_wasm_reference_types,omitempty" json:"enableWasmReferenceTypes,omitempty"`
	// Configures whether the WebAssembly SIMD proposal will be enabled for
	// compilation.  The WebAssembly SIMD proposal. This feature gates items
	// such as the v128 type and all of its operators being in a module. Note
	// that this does not enable the relaxed simd proposal.
	// This is false by default.
	EnableWasmSIMD bool `yaml:"enable_wasm_simd,omitempty" json:"enableWasmSIMD,omitempty"`
	// EnableDefaultCache enables compiled code caching for this `Config` using the default settings
	// configuration can be found.
	//
	// For more information about caching see
	// https://bytecodealliance.github.io/wasmtime/cli-cache.html
	// This is false by default.
	EnableDefaultCache bool `yaml:"enable_default_cache,omitempty" json:"enableDefaultCache,omitempty"`
	// EnableTestingOnlyMode enables test mode which provides access to non production debugging features.
	// This should not be set for a live system as it has both performance and security considerations.
	// This is false by default.
	EnableTestingOnlyMode bool `yaml:"enable_testing_only_mode,omitempty" json:"enableTestingOnlyMode,omitempty"`
	// SetMaxWasmStack configures the maximum stack size, in bytes, that JIT code can use.
	// The amount of stack space that wasm takes is always relative to the first invocation of wasm on the stack.
	// Recursive calls with host frames in the middle will all need to fit within this setting.
	// Note that this setting is not interpreted with 100% precision.
	// This is 256 MiB by default.
	MaxWasmStack int `yaml:"max_wasm_stack,omitempty" json:"maxWasmStack,omitempty"`
	// LimitMaxMemory defines the maximum number of pages of memory that can be used.
	// Each page represents 64KiB of memory.
	// This is 18 pages by default.
	LimitMaxMemory int64 `yaml:"limit_max_memory,omitempty" json:"limitMaxMemory,omitempty"`
	// ProfilingStrategy decides what sort of profiling to enable, if any.
	// Default is `wasmtime.ProfilingStrategyNone`.
	ProfilingStrategy wasmtime.ProfilingStrategy
	// CompileStrategy helps the engine to understand if the files has been precompiled.
	CompileStrategy EngineCompileStrategy

	err wrappers.Errs
}

// WithCompileStrategy defines the EngineCompileStrategy.
//
// Default is CompileWasm.
func (c *ConfigBuilder) WithCompileStrategy(strategy EngineCompileStrategy) *ConfigBuilder {
	c.CompileStrategy = strategy
	return c
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

// WithBulkMemory enables`memory.copy` instruction, tables and passive data.
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

// WithLimitMaxMemory defines the maximum number of pages of memory that can be used.
// Each page represents 64KiB of memory.
//
// Default is 18 pages.
func (c *ConfigBuilder) WithLimitMaxMemory(max uint64) *ConfigBuilder {
	if max > math.MaxInt64 {
		c.err.Add(fmt.Errorf("max memory %d is greater than max int64 %d", max, math.MaxInt64))
	} else {
		c.LimitMaxMemory = int64(max)
	}
	return c
}

// WithDefaultCache enables the default caching strategy.
//
// Default is false.
func (c *ConfigBuilder) WithDefaultCache(enabled bool) *ConfigBuilder {
	c.EnableDefaultCache = enabled
	return c
}

// WithEnableTestingOnlyMode enables test mode which provides access to
// useful debugging information. This should not be set for a live
// system as it has both performance and security considerations.
//
// Note: This requires Rust programs to be compiled with the  Wasm to be
// compiled with the wasm32-wasi target.
//
// Default is false.
func (c *ConfigBuilder) WithEnableTestingOnlyMode(enabled bool) *ConfigBuilder {
	c.EnableTestingOnlyMode = enabled
	return c
}

// build is private to ensure that the config id built
func (c *ConfigBuilder) Build() (*Config, error) {
	// validate inputs
	if c.err.Errored() {
		return nil, c.err.Err
	}

	return &Config{
		// engine config
		enableBulkMemory:         c.EnableBulkMemory,
		enableWasmMultiValue:     c.EnableWasmMultiValue,
		enableWasmReferenceTypes: c.EnableWasmReferenceTypes,
		enableWasmSIMD:           c.EnableWasmSIMD,
		enableDefaultCache:       c.EnableDefaultCache,
		maxWasmStack:             c.MaxWasmStack,
		profilingStrategy:        c.ProfilingStrategy,

		// limits
		limitMaxMemory:        c.LimitMaxMemory,
		limitMaxTableElements: defaultLimitMaxTableElements,
		limitMaxTables:        defaultLimitMaxTables,
		limitMaxInstances:     defaultLimitMaxInstances,
		limitMaxMemories:      defaultLimitMaxMemories,

		// runtime config
		compileStrategy: c.CompileStrategy,
		testingOnlyMode: c.EnableTestingOnlyMode,
	}, nil
}

// Engine returns a new wasmtime engine config. A wasmtime config can only be used once.
func (c *Config) Engine() (*wasmtime.Config, error) {
	cfg := defaultWasmtimeConfig()
	if c.enableDefaultCache {
		err := cfg.CacheConfigLoadDefault()
		if err != nil {
			return nil, err
		}
	}

	cfg.SetWasmBulkMemory(c.enableBulkMemory)
	cfg.SetWasmMultiValue(c.enableWasmMultiValue)
	cfg.SetWasmReferenceTypes(c.enableWasmReferenceTypes)
	cfg.SetWasmSIMD(c.enableWasmSIMD)
	cfg.SetProfiler(c.profilingStrategy)
	cfg.SetMaxWasmStack(c.maxWasmStack)

	return cfg, nil
}

type Config struct {
	enableBulkMemory         bool
	enableWasmMultiValue     bool
	enableWasmReferenceTypes bool
	enableWasmSIMD           bool
	enableDefaultCache       bool
	maxWasmStack             int
	profilingStrategy        wasmtime.ProfilingStrategy

	// store limits
	limitMaxMemory        int64
	limitMaxTableElements int64
	limitMaxTables        int64
	// number of instances of this module can be instantiated in parallel
	limitMaxInstances int64
	limitMaxMemories  int64

	testingOnlyMode bool

	compileStrategy EngineCompileStrategy
}

func defaultWasmtimeConfig() *wasmtime.Config {
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
