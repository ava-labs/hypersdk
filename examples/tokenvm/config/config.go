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
	"github.com/ava-labs/hypersdk/config"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/examples/tokenvm/version"
)

var _ vm.Config = (*Config)(nil)

const (
	defaultContinuousProfilerFrequency = 1 * time.Minute
	defaultContinuousProfilerMaxFiles  = 10
	defaultStoreTransactions           = true
	defaultMaxOrdersPerPair            = 1024
)

type Config struct {
	*config.Config

	// Concurrency
	SignatureVerificationCores int `json:"signatureVerificationCores"`
	RootGenerationCores        int `json:"rootGenerationCores"`
	TransactionExecutionCores  int `json:"transactionExecutionCores"`

	// Gossip
	GossipMaxSize       int   `json:"gossipMaxSize"`
	GossipProposerDiff  int   `json:"gossipProposerDiff"`
	GossipProposerDepth int   `json:"gossipProposerDepth"`
	NoGossipBuilderDiff int   `json:"noGossipBuilderDiff"`
	VerifyTimeout       int64 `json:"verifyTimeout"`

	// Tracing
	TraceEnabled    bool    `json:"traceEnabled"`
	TraceSampleRate float64 `json:"traceSampleRate"`

	// Profiling
	ContinuousProfilerDir string `json:"continuousProfilerDir"` // "*" is replaced with rand int

	// Streaming settings
	StreamingBacklogSize int `json:"streamingBacklogSize"`

	// Mempool
	MempoolSize         int      `json:"mempoolSize"`
	MempoolPayerSize    int      `json:"mempoolPayerSize"`
	MempoolExemptPayers []string `json:"mempoolExemptPayers"`

	// Order Book
	//
	// This is denoted as <asset 1>-<asset 2>
	MaxOrdersPerPair int      `json:"maxOrdersPerPair"`
	TrackedPairs     []string `json:"trackedPairs"` // which asset ID pairs we care about

	// Misc
	VerifySignatures  bool          `json:"verifySignatures"`
	StoreTransactions bool          `json:"storeTransactions"`
	TestMode          bool          `json:"testMode"` // makes gossip/building manual
	LogLevel          logging.Level `json:"logLevel"`

	// State Sync
	StateSyncServerDelay time.Duration `json:"stateSyncServerDelay"` // for testing

	loaded             bool
	nodeID             ids.NodeID
	parsedExemptPayers [][]byte
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

	// Parse any exempt payers (usually used when a single account is
	// broadcasting many txs at once)
	c.parsedExemptPayers = make([][]byte, len(c.MempoolExemptPayers))
	for i, payer := range c.MempoolExemptPayers {
		p, err := utils.ParseAddress(payer)
		if err != nil {
			return nil, err
		}
		c.parsedExemptPayers[i] = p[:]
	}
	return c, nil
}

func (c *Config) setDefault() {
	c.LogLevel = c.Config.GetLogLevel()
	gcfg := gossiper.DefaultProposerConfig()
	c.GossipMaxSize = gcfg.GossipMaxSize
	c.GossipProposerDiff = gcfg.GossipProposerDiff
	c.GossipProposerDepth = gcfg.GossipProposerDepth
	c.NoGossipBuilderDiff = gcfg.NoGossipBuilderDiff
	c.VerifyTimeout = gcfg.VerifyTimeout
	c.SignatureVerificationCores = c.Config.GetSignatureVerificationCores()
	c.RootGenerationCores = c.Config.GetRootGenerationCores()
	c.TransactionExecutionCores = c.Config.GetTransactionExecutionCores()
	c.MempoolSize = c.Config.GetMempoolSize()
	c.MempoolPayerSize = c.Config.GetMempoolPayerSize()
	c.StateSyncServerDelay = c.Config.GetStateSyncServerDelay()
	c.StreamingBacklogSize = c.Config.GetStreamingBacklogSize()
	c.VerifySignatures = c.Config.GetVerifySignatures()
	c.StoreTransactions = defaultStoreTransactions
	c.MaxOrdersPerPair = defaultMaxOrdersPerPair
}

func (c *Config) GetLogLevel() logging.Level         { return c.LogLevel }
func (c *Config) GetTestMode() bool                  { return c.TestMode }
func (c *Config) GetSignatureVerificationCores() int { return c.SignatureVerificationCores }
func (c *Config) GetRootGenerationCores() int        { return c.RootGenerationCores }
func (c *Config) GetTransactionExecutionCores() int  { return c.TransactionExecutionCores }
func (c *Config) GetMempoolSize() int                { return c.MempoolSize }
func (c *Config) GetMempoolPayerSize() int           { return c.MempoolPayerSize }
func (c *Config) GetMempoolExemptPayers() [][]byte   { return c.parsedExemptPayers }
func (c *Config) GetTraceConfig() *trace.Config {
	return &trace.Config{
		Enabled:         c.TraceEnabled,
		TraceSampleRate: c.TraceSampleRate,
		AppName:         consts.Name,
		Agent:           c.nodeID.String(),
		Version:         version.Version.String(),
	}
}
func (c *Config) GetStateSyncServerDelay() time.Duration { return c.StateSyncServerDelay }
func (c *Config) GetStreamingBacklogSize() int           { return c.StreamingBacklogSize }
func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	if len(c.ContinuousProfilerDir) == 0 {
		return &profiler.Config{Enabled: false}
	}
	// Replace all instances of "*" with nodeID. This is useful when
	// running multiple instances of tokenvm on the same machine.
	c.ContinuousProfilerDir = strings.ReplaceAll(c.ContinuousProfilerDir, "*", c.nodeID.String())
	return &profiler.Config{
		Enabled:     true,
		Dir:         c.ContinuousProfilerDir,
		Freq:        defaultContinuousProfilerFrequency,
		MaxNumFiles: defaultContinuousProfilerMaxFiles,
	}
}
func (c *Config) GetVerifySignatures() bool  { return c.VerifySignatures }
func (c *Config) GetStoreTransactions() bool { return c.StoreTransactions }
func (c *Config) Loaded() bool               { return c.loaded }
