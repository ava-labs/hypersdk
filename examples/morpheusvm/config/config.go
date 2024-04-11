// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/config"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/version"
)

var _ vm.Config = (*Config)(nil)

const (
	defaultContinuousProfilerFrequency = 1 * time.Minute
	defaultContinuousProfilerMaxFiles  = 10
)

type Config struct {
	*config.Config

	// Building
	ChunkBuildFrequency      int64 `json:"chunkBuildFrequency"`      // in milliseconds
	TargetChunkBuildDuration int64 `json:"targetChunkBuildDuration"` // in milliseconds
	BlockBuildFrequency      int64 `json:"blockBuildFrequency"`      // in milliseconds

	// Concurrency
	AuthExecutionCores   int `json:"authExecutionCores"`
	PrecheckCores        int `json:"precheckCores"`
	ActionExecutionCores int `json:"actionExecutionCores"`
	MissingChunkFetchers int `json:"missingChunkFetchers"`
	AuthGossipCores      int `json:"authGossipCores"`
	AuthGossipBacklog    int `json:"authGossipBacklog"`
	AuthRPCCores         int `json:"authRPCCores"`
	AuthRPCBacklog       int `json:"authRPCBacklog"`
	ChunkStorageCores    int `json:"chunkStorageCores"`
	ChunkStorageBacklog  int `json:"chunkStorageBacklog"`

	// Tracing
	TraceEnabled    bool    `json:"traceEnabled"`
	TraceSampleRate float64 `json:"traceSampleRate"`

	// Profiling
	ContinuousProfilerDir string `json:"continuousProfilerDir"` // "*" is replaced with rand int

	// Streaming settings
	StreamingBacklogSize int `json:"streamingBacklogSize"`

	// Mempool
	MempoolSize           int      `json:"mempoolSize"`
	MempoolSponsorSize    int      `json:"mempoolSponsorSize"`
	MempoolExemptSponsors []string `json:"mempoolExemptSponsors"`

	// Misc
	VerifyAuth bool          `json:"verifyAuth"`
	TestMode   bool          `json:"testMode"` // makes gossip/building manual
	LogLevel   logging.Level `json:"logLevel"`

	loaded               bool
	nodeID               ids.NodeID
	parsedExemptSponsors []codec.Address
}

func New(nodeID ids.NodeID, b []byte) (*Config, error) {
	c := &Config{nodeID: nodeID}
	c.setDefault()
	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
		c.loaded = true
	}

	// Parse any exempt sponsors (usually used when a single account is
	// broadcasting many txs at once)
	c.parsedExemptSponsors = make([]codec.Address, len(c.MempoolExemptSponsors))
	for i, sponsor := range c.MempoolExemptSponsors {
		p, err := codec.ParseAddressBech32(consts.HRP, sponsor)
		if err != nil {
			return nil, err
		}
		c.parsedExemptSponsors[i] = p
	}
	return c, nil
}

func (c *Config) setDefault() {
	c.LogLevel = c.Config.GetLogLevel()
	c.ChunkBuildFrequency = c.Config.GetChunkBuildFrequency().Milliseconds()
	c.TargetChunkBuildDuration = c.Config.GetTargetChunkBuildDuration().Milliseconds()
	c.BlockBuildFrequency = c.Config.GetBlockBuildFrequency().Milliseconds()
	c.AuthExecutionCores = c.Config.GetAuthExecutionCores()
	c.PrecheckCores = c.Config.GetPrecheckCores()
	c.ActionExecutionCores = c.Config.GetActionExecutionCores()
	c.MissingChunkFetchers = c.Config.GetMissingChunkFetchers()
	c.MempoolSize = c.Config.GetMempoolSize()
	c.MempoolSponsorSize = c.Config.GetMempoolSponsorSize()
	c.StreamingBacklogSize = c.Config.GetStreamingBacklogSize()
	c.VerifyAuth = c.Config.GetVerifyAuth()
	c.AuthRPCCores = c.Config.GetAuthRPCCores()
	c.AuthRPCBacklog = c.Config.GetAuthRPCBacklog()
	c.AuthGossipCores = c.Config.GetAuthGossipCores()
	c.AuthGossipBacklog = c.Config.GetAuthGossipBacklog()
	c.ChunkStorageCores = c.Config.GetChunkStorageCores()
	c.ChunkStorageBacklog = c.Config.GetChunkStorageBacklog()
}

func (c *Config) GetLogLevel() logging.Level                { return c.LogLevel }
func (c *Config) GetTestMode() bool                         { return c.TestMode }
func (c *Config) GetAuthExecutionCores() int                { return c.AuthExecutionCores }
func (c *Config) GetActionExecutionCores() int              { return c.ActionExecutionCores }
func (c *Config) GetMempoolSize() int                       { return c.MempoolSize }
func (c *Config) GetMempoolSponsorSize() int                { return c.MempoolSponsorSize }
func (c *Config) GetMempoolExemptSponsors() []codec.Address { return c.parsedExemptSponsors }
func (c *Config) GetTraceConfig() *trace.Config {
	return &trace.Config{
		Enabled:         c.TraceEnabled,
		TraceSampleRate: c.TraceSampleRate,
		AppName:         consts.Name,
		Agent:           c.nodeID.String(),
		Version:         version.Version.String(),
	}
}
func (c *Config) GetStreamingBacklogSize() int { return c.StreamingBacklogSize }
func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	if len(c.ContinuousProfilerDir) == 0 {
		return &profiler.Config{Enabled: false}
	}
	// Replace all instances of "*" with nodeID. This is useful when
	// running multiple instances of morpheusvm on the same machine.
	c.ContinuousProfilerDir = strings.ReplaceAll(c.ContinuousProfilerDir, "*", c.nodeID.String())
	return &profiler.Config{
		Enabled:     true,
		Dir:         c.ContinuousProfilerDir,
		Freq:        defaultContinuousProfilerFrequency,
		MaxNumFiles: defaultContinuousProfilerMaxFiles,
	}
}
func (c *Config) GetVerifyAuth() bool { return c.VerifyAuth }
func (c *Config) Loaded() bool        { return c.loaded }
func (c *Config) GetBeneficiary() codec.Address {
	return codec.BlackholeAddress
}

func (c *Config) GetChunkBuildFrequency() time.Duration {
	return time.Duration(c.ChunkBuildFrequency) * time.Millisecond
}

func (c *Config) GetTargetChunkBuildDuration() time.Duration {
	return time.Duration(c.TargetChunkBuildDuration) * time.Millisecond
}

func (c *Config) GetBlockBuildFrequency() time.Duration {
	return time.Duration(c.BlockBuildFrequency) * time.Millisecond
}

func (c *Config) GetAuthRPCCores() int {
	return c.AuthRPCCores
}

func (c *Config) GetAuthRPCBacklog() int {
	return c.AuthRPCBacklog
}

func (c *Config) GetMissingChunkFetchers() int {
	return c.MissingChunkFetchers
}

func (c *Config) GetAuthGossipCores() int {
	return c.AuthGossipCores
}

func (c *Config) GetAuthGossipBacklog() int {
	return c.AuthGossipBacklog
}

func (c *Config) GetPrecheckCores() int {
	return c.PrecheckCores
}

func (c *Config) GetChunkStorageCores() int {
	return c.ChunkStorageCores
}

func (c *Config) GetChunkStorageBacklog() int {
	return c.ChunkStorageBacklog
}
