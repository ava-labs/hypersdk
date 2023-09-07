// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:revive
package config

import (
	"runtime"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/trace"
)

const avalancheGoMinCPU = 4

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
func (c *Config) GetStreamingBacklogSize() int           { return 1024 }
func (c *Config) GetStateEvictionBatchSize() int         { return 4 * units.MiB }
func (c *Config) GetIntermediateNodeCacheSize() int      { return 2 * units.GiB }
func (c *Config) GetValueNodeCacheSize() int             { return 2 * units.GiB }
func (c *Config) GetTraceConfig() *trace.Config          { return &trace.Config{Enabled: false} }
func (c *Config) GetStateSyncParallelism() int           { return 4 }
func (c *Config) GetStateSyncServerDelay() time.Duration { return 0 } // used for testing

func (c *Config) GetParsedBlockCacheSize() int  { return 128 }
func (c *Config) GetStateHistoryLength() int    { return 256 }
func (c *Config) GetAcceptedBlockWindow() int   { return 768 }
func (c *Config) GetStateSyncMinBlocks() uint64 { return 768 }
func (c *Config) GetAcceptorSize() int          { return 1024 }

func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	return &profiler.Config{Enabled: false}
}
func (c *Config) GetVerifySignatures() bool              { return true }
func (c *Config) GetTargetBuildDuration() time.Duration  { return 100 * time.Millisecond }
func (c *Config) GetProcessingBuildSkip() int            { return 16 }
func (c *Config) GetTargetGossipDuration() time.Duration { return 20 * time.Millisecond }
func (c *Config) GetBlockCompactionFrequency() int       { return 32 } // 64 MB of deletion if 2 MB blocks
