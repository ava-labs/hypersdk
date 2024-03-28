// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:revive
package config

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/trace"
)

type Config struct{}

func (c *Config) GetLogLevel() logging.Level                { return logging.Info }
func (c *Config) GetAuthExecutionCores() int                { return 1 }
func (c *Config) GetAuthRPCCores() int                      { return 1 }
func (c *Config) GetAuthRPCBacklog() int                    { return 1_024 }
func (c *Config) GetAuthGossipCores() int                   { return 1 }
func (c *Config) GetAuthGossipBacklog() int                 { return 1_024 }
func (c *Config) GetRootGenerationCores() int               { return 1 }
func (c *Config) GetPrecheckCores() int                     { return 1 }
func (c *Config) GetActionExecutionCores() int              { return 1 }
func (c *Config) GetMissingChunkFetchers() int              { return 4 }
func (c *Config) GetMempoolSize() int                       { return 2 * units.GiB }
func (c *Config) GetMempoolSponsorSize() int                { return 32 }
func (c *Config) GetMempoolExemptSponsors() []codec.Address { return nil }
func (c *Config) GetStreamingBacklogSize() int              { return 1_024 }
func (c *Config) GetIntermediateNodeCacheSize() int         { return 4 * units.GiB }
func (c *Config) GetStateIntermediateWriteBufferSize() int  { return 32 * units.MiB }
func (c *Config) GetStateIntermediateWriteBatchSize() int   { return 4 * units.MiB }
func (c *Config) GetValueNodeCacheSize() int                { return 2 * units.GiB }
func (c *Config) GetTraceConfig() *trace.Config             { return &trace.Config{Enabled: false} }
func (c *Config) GetStateSyncParallelism() int              { return 4 }
func (c *Config) GetStateSyncServerDelay() time.Duration    { return 0 } // used for testing

func (c *Config) GetParsedBlockCacheSize() int     { return 128 }
func (c *Config) GetStateHistoryLength() int       { return 3 }   // 3 minutes of state history (root generated once per minute, just need to get to latest to keep syncing)
func (c *Config) GetAcceptedBlockWindowCache() int { return 128 } // 256MB at 2MB blocks
func (c *Config) GetAcceptedBlockWindow() int      { return 512 } // TODO: make this longer for prod
func (c *Config) GetStateSyncMinBlocks() uint64    { return 768 } // set to max int for archive nodes to ensure no skips
func (c *Config) GetAcceptorSize() int             { return 64 }

func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	return &profiler.Config{Enabled: false}
}
func (c *Config) GetVerifyAuth() bool                             { return true }
func (c *Config) GetTargetChunkBuildDuration() time.Duration      { return 100 * time.Millisecond }
func (c *Config) GetChunkBuildFrequency() time.Duration           { return 250 * time.Millisecond }
func (c *Config) GetBlockBuildFrequency() time.Duration           { return time.Second }
func (c *Config) GetProcessingBuildSkip() int                     { return 16 }
func (c *Config) GetBlockCompactionFrequency() int                { return 32 } // 64 MB of deletion if 2 MB blocks
func (c *Config) GetMinimumCertificateBroadcastNumerator() uint64 { return 75 } // out of 100 (more weight == more fees)
func (c *Config) GetBeneficiary() codec.Address                   { return codec.EmptyAddress }
