// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:revive
package config

import (
	"runtime"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/AnomalyFi/hypersdk/trace"
	"github.com/AnomalyFi/hypersdk/vm"
)

const avalancheGoMinCPU = 4

var _ vm.Config = (*Config)(nil)

type Config struct{}

func (c *Config) GetLogLevel() logging.Level { return logging.Info }
func (c *Config) GetParallelism() int {
	numCPUs := runtime.NumCPU()
	if numCPUs > avalancheGoMinCPU {
		return numCPUs - avalancheGoMinCPU
	}
	return 1
}
func (c *Config) GetMempoolSize() int                    { return 2_048 }
func (c *Config) GetMempoolPayerSize() int               { return 32 }
func (c *Config) GetMempoolExemptPayers() [][]byte       { return nil }
func (c *Config) GetMempoolVerifyBalances() bool         { return true }
func (c *Config) GetStreamingBacklogSize() int           { return 1024 }
func (c *Config) GetStateHistoryLength() int             { return 256 }
func (c *Config) GetStateCacheSize() int                 { return 65_536 } // nodes
func (c *Config) GetAcceptorSize() int                   { return 1024 }
func (c *Config) GetTraceConfig() *trace.Config          { return &trace.Config{Enabled: false} }
func (c *Config) GetStateSyncParallelism() int           { return 4 }
func (c *Config) GetStateSyncMinBlocks() uint64          { return 256 }
func (c *Config) GetStateSyncServerDelay() time.Duration { return 0 } // used for testing
func (c *Config) GetBlockLRUSize() int                   { return 128 }

func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	return &profiler.Config{Enabled: false}
}
