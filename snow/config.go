// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/hypersdk/context"
)

const (
	VMConfigKey           = "vm"
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

func GetVMConfig(ctx *context.Context) (VMConfig, error) {
	return context.GetConfigFromContext(ctx, VMConfigKey, NewDefaultVMConfig())
}

func GetTraceConfig(ctx *context.Context) (trace.Config, error) {
	return context.GetConfigFromContext(ctx, TracerConfigKey, trace.Config{Enabled: false})
}

func GetProfilerConfig(ctx *context.Context) (profiler.Config, error) {
	return context.GetConfigFromContext(ctx, ContinuousProfilerKey, profiler.Config{Enabled: false})
}
