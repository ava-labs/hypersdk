// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/host"
)

type Config struct {
	// EnableDebugMode is a test mode which provides access to non production debugging features.
	// This should not be set for a live system as it has both performance and security considerations.
	// Note: This requires associated Engine to enable support for BulkMemory.
	// This is false by default.
	EnableDebugMode bool `json:"enableTestingOnlyMode,omitempty" yaml:"enable_testing_only_mode,omitempty"`
	// LimitMaxMemory defines the maximum number of pages of memory that can be used.
	// Each page represents 64KiB of memory.
	// This is 18 pages by default.
	LimitMaxMemory uint32 `json:"limitMaxMemory,omitempty" yaml:"limit_max_memory,omitempty"`
	// CompileStrategy helps the engine to understand if the files has been precompiled.
	CompileStrategy engine.CompileStrategy `json:"compileStrategy,omitempty" yaml:"compile_strategy,omitempty"`
	// ImportFnCallback is a global callback for all import function requests and responses.
	ImportFnCallback host.ImportFnCallback `json:"-" yaml:"-"`
}

// NewConfig returns a new runtime configuration with default settings.
func NewConfig() *Config {
	return &Config{
		LimitMaxMemory:   engine.DefaultLimitMaxMemory,
		CompileStrategy:  engine.DefaultCompileStrategy,
		EnableDebugMode:  false,
		ImportFnCallback: host.ImportFnCallback{},
	}
}

// SetCompileStrategy defines the EngineCompileStrategy.
//
// Default is CompileWasm.
func (c *Config) SetCompileStrategy(strategy engine.CompileStrategy) *Config {
	c.CompileStrategy = strategy
	return c
}

// SetLimitMaxMemory defines the maximum number of pages of memory that can be used.
// Each page represents 64KiB of memory.
//
// Default is 18 pages.
func (c *Config) SetLimitMaxMemory(max uint32) *Config {
	c.LimitMaxMemory = max
	return c
}

// SetDebugMode enables debug mode which provides access to useful debugging
// information. If [enabled] is true the associated `Engine` must enable
// BulkMemory to support useful IO operations during debugging.
//
// This should not be set for a live system as it has
// both performance and security considerations.
//
// Note: This requires Rust programs to be compiled with the wasm32-wasi target.
//
// Default is false.
func (c *Config) SetDebugMode(enabled bool) *Config {
	c.EnableDebugMode = enabled
	return c
}

// SetImportFnCallback sets the global import function callback.
func (c *Config) SetImportFnCallback(callback host.ImportFnCallback) *Config {
	c.ImportFnCallback = callback
	return c
}
