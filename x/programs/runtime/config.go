// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import "github.com/bytecodealliance/wasmtime-go/v13"

const (
	defaultMaxWasmStack                 = 256 * 1024 * 1024 // 256 MiB
	defaultWasmThreads                  = false
	defaultFuelMetering                 = true
	defaultWasmMultiMemory              = false
	defaultWasmMemory64                 = false
	defaultLimitMaxMemory               = 16 * 64 * 1024 // 16 pages
	defaultSIMD                         = false
	defaultCompilerStrategy             = wasmtime.StrategyCranelift
	defaultEpochInterruption            = true
	defaultNaNCanonicalization          = "true"
	defaultCraneliftOptLevel            = wasmtime.OptLevelSpeed
	defaultEnableReferenceTypes         = false
	defaultEnableBulkMemory             = false
	defaultProfiler                     = wasmtime.ProfilingStrategyNone
	defaultMultiValue                   = false
	defaultEnableCraneliftDebugVerifier = false
	defaultEnableDebugInfo              = false

	defaultLimitMaxTableElements = 4096
	defaultLimitMaxTables        = 1
	defaultLimitMaxInstances     = 32
	defaultLimitMaxMemories      = 1
)

func NewConfigBuilder(meterMaxUnits uint64) *builder {
	cfg := defaultWasmtimeConfig()
	return &builder{
		cfg:           cfg,
		meterMaxUnits: meterMaxUnits,
	}
}

type builder struct {
	cfg *wasmtime.Config

	// engine
	compileStrategy EngineCompileStrategy
	defaultCache    bool
	meterMaxUnits   uint64

	// limit
	limitMaxMemory int64
}

type Config struct {
	engine *wasmtime.Config

	// store limits
	limitMaxMemory        int64
	limitMaxTableElements int64
	limitMaxTables        int64
	// number of instances of this module can be instantiated in parallel
	limitMaxInstances int64
	limitMaxMemories  int64

	compileStrategy EngineCompileStrategy
	meterMaxUnits   uint64
}

// WithCompileStrategy defines the EngineCompileStrategy.
// Default is “.
func (b *builder) WithCompileStrategy(strategy EngineCompileStrategy) *builder {
	b.compileStrategy = strategy
	return b
}

// WithMaxWasmStack defines the maximum amount of stack space available for
// executing WebAssembly code.
//
// Default is 256 MiB.
func (b *builder) WithMaxWasmStack(max int) *builder {
	b.cfg.SetMaxWasmStack(max)
	return b
}

// WithMultiValue enables modules that can return multiple values.
//
// ref. https://github.com/webassembly/multi-value
// Default is false.
func (b *builder) WithMultiValue(enable bool) *builder {
	b.cfg.SetWasmMultiValue(enable)
	return b
}

// WithBulkMemory enables`memory.copy` instruction, tables and passive data.
//
// ref. https://github.com/WebAssembly/bulk-memory-operations
// Default is false.
func (b *builder) WithBulkMemory(enable bool) *builder {
	b.cfg.SetWasmBulkMemory(enable)
	return b
}

// WithReferenceTypes Enables the `externref` and `funcref` types as well as
// allowing a module to define multiple tables.
//
// ref. https://github.com/webassembly/reference-types
//
// Note: depends on bulk memory being enabled.
// Default is false.
func (b *builder) WithReferenceTypes(enable bool) *builder {
	b.cfg.SetWasmReferenceTypes(enable)
	return b
}

// WithSIMD enables SIMD instructions including v128.
//
// ref. https://github.com/webassembly/simd
// Default is false.
func (b *builder) WithSIMD(enable bool) *builder {
	b.cfg.SetWasmSIMD(enable)
	return b
}

// WithProfilingStrategy defines the profiling strategy used for defining the
// default profiler.
//
// Default is `wasmtime.ProfilingStrategyNone`.
func (b *builder) WithProfilingStrategy(strategy wasmtime.ProfilingStrategy) *builder {
	b.cfg.SetProfiler(strategy)
	return b
}

// WithLimitMaxMemory defines the maximum number of pages of memory that can be used.
// Each page represents 64KiB of memory.
//
// Default is 16 pages.
func (b *builder) WithLimitMaxMemory(max int64) *builder {
	b.limitMaxMemory = max
	return b
}

// WithDefaultCache enables the default caching strategy.
//
// Default is false.
func (b *builder) WithDefaultCache(enabled bool) *builder {
	b.defaultCache = enabled
	return b
}

func (b *builder) Build() (*Config, error) {
	if b.defaultCache {
		err := b.cfg.CacheConfigLoadDefault()
		if err != nil {
			return nil, err
		}
	}

	if b.limitMaxMemory == 0 {
		b.limitMaxMemory = defaultLimitMaxMemory
	}

	return &Config{
		// engine config
		engine: b.cfg,

		// limits
		limitMaxTableElements: defaultLimitMaxTableElements,
		limitMaxMemory:        b.limitMaxMemory,
		limitMaxTables:        defaultLimitMaxTables,
		limitMaxInstances:     defaultLimitMaxInstances,
		limitMaxMemories:      defaultLimitMaxMemories,

		// runtime config
		compileStrategy: b.compileStrategy,
		meterMaxUnits:   b.meterMaxUnits,
	}, nil
}

// non-configurable defaults
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

	// configurable defaults
	cfg.SetWasmSIMD(defaultSIMD)
	cfg.SetMaxWasmStack(defaultMaxWasmStack)
	cfg.SetWasmBulkMemory(defaultEnableBulkMemory)
	cfg.SetWasmReferenceTypes(defaultEnableReferenceTypes)
	cfg.SetWasmMultiValue(defaultMultiValue)
	cfg.SetProfiler(defaultProfiler)
	return cfg
}
