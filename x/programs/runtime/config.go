// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"math"

	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/x/programs/engine"
)

// NewConfig returns a new runtime configuration with default settings.
// All instances of Config should be created with this constructor.
func NewConfig() *Config {
	return &Config{
		LimitMaxMemory:  engine.DefaultLimitMaxMemory,
		CompileStrategy: engine.DefaultCompileStrategy,
		EnableDebugMode: false,
	}
}

type Config struct {
	// EnableDebugMode is a test mode which provides access to non production debugging features.
	// This should not be set for a live system as it has both performance and security considerations.
	// This is false by default.
	EnableDebugMode bool `yaml:"enable_testing_only_mode,omitempty" json:"enableTestingOnlyMode,omitempty"`
	// LimitMaxMemory defines the maximum number of pages of memory that can be used.
	// Each page represents 64KiB of memory.
	// This is 18 pages by default.
	LimitMaxMemory int64 `yaml:"limit_max_memory,omitempty" json:"limitMaxMemory,omitempty"`
	// CompileStrategy helps the engine to understand if the files has been precompiled.
	CompileStrategy engine.CompileStrategy `yaml:"compile_strategy,omitempty" json:"compileStrategy,omitempty"`

	err wrappers.Errs
}

// WithCompileStrategy defines the EngineCompileStrategy.
//
// Default is CompileWasm.
func (c *Config) WithCompileStrategy(strategy engine.CompileStrategy) *Config {
	c.CompileStrategy = strategy
	return c
}

// WithLimitMaxMemory defines the maximum number of pages of memory that can be used.
// Each page represents 64KiB of memory.
//
// Default is 18 pages.
func (c *Config) WithLimitMaxMemory(max uint64) *Config {
	if max > math.MaxInt64 {
		c.err.Add(fmt.Errorf("max memory %d is greater than max int64 %d", max, math.MaxInt64))
	} else {
		c.LimitMaxMemory = int64(max)
	}
	return c
}

// WithDebugMode enables debug mode which provides access to
// useful debugging information. If [enabled] is true we set
// EnableBulkMemory to support useful IO operations during debugging.
//
// This should not be set for a live system as it has
// both performance and security considerations.
//
// Note: This requires Rust programs to be compiled with the wasm32-wasi target.
//
// Default is false.
func (c *Config) WithDebugMode(enabled bool) *Config {
	c.EnableDebugMode = enabled
	return c
}

func (c *Config) Build() (*Config, error) {
	return c, c.err.Err
}
