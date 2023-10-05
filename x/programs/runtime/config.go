// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import "github.com/bytecodealliance/wasmtime-go/v13"

const (
	defaultMaxWasmStack    = 256 * 1024 * 1024 // 256 MiB
	defaultWasmThreads     = false
	defaultFuelMetering    = true
	defaultWasmMultiMemory = false
	defaultWasmMemory64    = false
	defaultLimitMaxMemory  = 16 * 64 * 1024 // 16 pages
)

// TODO: review Cranelift fine tune knobs when exposed
// https://github.com/bytecodealliance/wasmtime-go/pull/188

func NewConfigBuilder(meterMaxUnits uint64) *builder {
	return &builder{meterMaxUnits: meterMaxUnits}
}

type builder struct {
	// config
	maxWasmStack      int
	bulkMemory        bool
	multiValue        bool
	referenceTypes    bool
	simd              bool
	profilingStrategy wasmtime.ProfilingStrategy
	defaultCache      bool

	// engine
	compileStrategy   EngineCompileStrategy
	meterMaxUnits     uint64
	contextSwitchCost uint64

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

	compileStrategy   EngineCompileStrategy
	meterMaxUnits     uint64
	contextSwitchCost uint64
}

// WithCompileStrategy defines the EngineCompileStrategy.
// Default is CompileWasm“.
func (b *builder) WithCompileStrategy(strategy EngineCompileStrategy) *builder {
	b.compileStrategy = strategy
	return b
}

// WithMaxWasmStack defines the maximum amount of stack space available for
// executing WebAssembly code.
//
// Default is 256 MiB.
func (b *builder) WithMaxWasmStack(max int) *builder {
	b.maxWasmStack = max
	return b
}

// WithMultiValue enables modules that can return multiple values.
//
// ref. https://github.com/webassembly/multi-value
func (b *builder) WithMultiValue(enable bool) *builder {
	b.multiValue = enable
	return b
}

// WithBulkMemory enables`memory.copy` instruction, tables and passive data.
//
// ref. https://github.com/WebAssembly/bulk-memory-operations
func (b *builder) WithBulkMemory(enable bool) *builder {
	b.bulkMemory = enable
	return b
}

// WithReferenceTypes Enables the `externref` and `funcref` types as well as
// allowing a module to define multiple tables.
//
// ref. https://github.com/webassembly/reference-types
//
// Note: depends on bulk memory being enabled.
func (b *builder) WithReferenceTypes(enable bool) *builder {
	b.referenceTypes = enable
	return b
}

// WithSIMD enables SIMD instructions including v128.
//
// ref. https://github.com/webassembly/simd
func (b *builder) WithSIMD(enable bool) *builder {
	b.simd = enable
	return b
}

// WithProfilingStrategy defines the profiling strategy used for defining the
// default profiler.
//
// Default is `wasmtime.ProfilingStrategyNone`.
func (b *builder) WithProfilingStrategy(strategy wasmtime.ProfilingStrategy) *builder {
	b.profilingStrategy = strategy
	return b
}

// WithLimitMaxMemory defines the maximum number of pages of memory that can be used.
// Each page represents 64KiB of memory.
func (b *builder) WithLimitMaxMemory(max int64) *builder {
	b.limitMaxMemory = max
	return b
}

// WithDefaultCache enables the default caching strategy.
func (b *builder) WithDefaultCache(enabled bool) *builder {
	b.defaultCache = enabled
	return b
}

// WithContextSwitchCost defines the cost of a context switch in units.
func (b *builder) WithContextSwitchCost(units uint64) *builder {
	b.contextSwitchCost = units
	return b
}

func (b *builder) Build() (*Config, error) {
	cfg := defaultWasmtimeConfig()
	if b.maxWasmStack == 0 {
		b.maxWasmStack = defaultMaxWasmStack
	}
	cfg.SetMaxWasmStack(b.maxWasmStack)
	cfg.SetWasmBulkMemory(b.bulkMemory)
	cfg.SetWasmMultiValue(b.multiValue)
	cfg.SetWasmReferenceTypes(b.referenceTypes)
	cfg.SetWasmSIMD(b.simd)
	cfg.SetProfiler(b.profilingStrategy)
	if b.defaultCache {
		err := cfg.CacheConfigLoadDefault()
		if err != nil {
			return nil, err
		}
	}

	if b.limitMaxMemory == 0 {
		b.limitMaxMemory = defaultLimitMaxMemory
	}

	return &Config{
		engine: cfg,

		limitMaxTableElements: 8192,
		limitMaxMemory:        b.limitMaxMemory,
		limitMaxTables:        1,
		limitMaxInstances:     32,
		limitMaxMemories:      1,
		compileStrategy:       b.compileStrategy,
		meterMaxUnits:         b.meterMaxUnits,
		contextSwitchCost:     b.contextSwitchCost,
	}, nil
}

// non-configurable defaults
func defaultWasmtimeConfig() *wasmtime.Config {
	cfg := wasmtime.NewConfig()
	// defaults
	cfg.SetCraneliftOptLevel(wasmtime.OptLevelSpeedAndSize)
	cfg.SetConsumeFuel(defaultFuelMetering)
	cfg.SetWasmThreads(defaultWasmThreads)
	cfg.SetWasmMultiMemory(defaultWasmMultiMemory)
	cfg.SetWasmMemory64(defaultWasmMemory64)
	cfg.SetStrategy(wasmtime.StrategyCranelift)
	cfg.SetEpochInterruption(true)
	cfg.SetCraneliftFlag("enable_nan_canonicalization", "true")
	return cfg
}
