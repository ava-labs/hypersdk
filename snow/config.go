// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/profiler"

	"github.com/ava-labs/hypersdk/context"
)

const (
	SnowVMConfigKey       = "snowvm"
	TracerConfigKey       = "tracer"
	ContinuousProfilerKey = "continuousProfiler"
)

type VMConfig struct {
	ParsedBlockCacheSize     int `json:"parsedBlockCacheSize"`
	AcceptedBlockWindowCache int `json:"acceptedBlockWindowCache"`
}

func NewDefaultVMConfig() VMConfig {
	return VMConfig{
		ParsedBlockCacheSize:     128,
		AcceptedBlockWindowCache: 128,
	}
}

func GetVMConfig(config context.Config) (VMConfig, error) {
	return context.GetConfig(config, SnowVMConfigKey, NewDefaultVMConfig())
}

func GetProfilerConfig(config context.Config) (profiler.Config, error) {
	return context.GetConfig(config, ContinuousProfilerKey, profiler.Config{Enabled: false})
}

func GetTracerConfig(config context.Config) (trace.Config, error) {
	return context.GetConfig(config, TracerConfigKey, trace.Config{Enabled: false})
}
