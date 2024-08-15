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
func (c *Config) GetPrecheckCores() int                     { return 1 }
func (c *Config) GetActionExecutionCores() int              { return 1 }
func (c *Config) GetChunkStorageCores() int                 { return 1 }
func (c *Config) GetChunkStorageBacklog() int               { return 1_024 }
func (c *Config) GetMissingChunkFetchers() int              { return 4 }
func (c *Config) GetMempoolSize() int                       { return 2 * units.GiB }
func (c *Config) GetMempoolSponsorSize() int                { return 32 }
func (c *Config) GetMempoolExemptSponsors() []codec.Address { return nil }
func (c *Config) GetStreamingBacklogSize() int              { return 1_024 }
func (c *Config) GetTraceConfig() *trace.Config             { return &trace.Config{Enabled: false} }
func (c *Config) GetParsedBlockCacheSize() int              { return 128 }
func (c *Config) GetAcceptedBlockWindowCache() int          { return 128 } // 256MB at 2MB blocks
func (c *Config) GetAcceptedBlockWindow() int               { return 512 } // TODO: make this longer for prod
func (c *Config) GetAcceptorSize() int                      { return 64 }

func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	return &profiler.Config{Enabled: false}
}
func (c *Config) GetVerifyAuth() bool                             { return true }
func (c *Config) GetTargetChunkBuildDuration() time.Duration      { return 100 * time.Millisecond }
func (c *Config) GetChunkBuildFrequency() time.Duration           { return 250 * time.Millisecond }
func (c *Config) GetBlockBuildFrequency() time.Duration           { return time.Second }
func (c *Config) GetProcessingBuildSkip() int                     { return 16 }
func (c *Config) GetMinimumCertificateBroadcastNumerator() uint64 { return 85 } // out of 100 (more weight == more fees) // @todo
func (c *Config) GetBeneficiary() codec.Address                   { return codec.EmptyAddress }
func (c *Config) GetAnchorURL() string                            { return "" }
