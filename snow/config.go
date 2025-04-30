// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/profiler"

	"github.com/ava-labs/hypersdk/context"
)

const (
	// SnowVMConfigKey is the key for the snow VM config
	SnowVMConfigKey = "snowvm"

	// TracerConfigKey is the key for the tracer config
	TracerConfigKey = "tracer"

	// ContinuousProfilerKey is the key for the continuous profiler config
	ContinuousProfilerKey = "continuousProfiler"
)

// VMConfig contains configuration for parsed block cache size and accepted block window cache size
type VMConfig struct {
	ParsedBlockCacheSize     int `json:"parsedBlockCacheSize"`
	AcceptedBlockWindowCache int `json:"acceptedBlockWindowCache"`
}

// NewDefaultVMConfig returns a default VMConfig
func NewDefaultVMConfig() VMConfig {
	return VMConfig{
		ParsedBlockCacheSize:     128,
		AcceptedBlockWindowCache: 128,
	}
}

// GetVMConfig returns the VMConfig from the context.Config. If the config does not contain a VMConfig,
// it returns a default VMConfig.
func GetVMConfig(config context.Config) (VMConfig, error) {
	return context.GetConfig(config, SnowVMConfigKey, NewDefaultVMConfig())
}

// GetProfilerConfig returns the profiler.Config. If the config does not contain a profiler.Config,
// it disables profiling.
func GetProfilerConfig(config context.Config) (profiler.Config, error) {
	return context.GetConfig(config, ContinuousProfilerKey, profiler.Config{Enabled: false})
}

// GetTracerConfig returns the trace.Config. If the config does not contain a trace.Config,
// it disables tracing.
func GetTracerConfig(config context.Config) (trace.Config, error) {
	return context.GetConfig(config, TracerConfigKey, trace.Config{Enabled: false})
}
