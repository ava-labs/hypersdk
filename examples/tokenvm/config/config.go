// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/config"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/examples/tokenvm/version"
)

var _ vm.Config = (*Config)(nil)

type Config struct {
	*config.Config

	// Tracing
	TraceEnabled    bool    `json:"traceEnabled"`
	TraceSampleRate float64 `json:"traceSampleRate"`

	// Streaming Ports
	DecisionsPort uint16 `json:"decisionsPort"`
	BlocksPort    uint16 `json:"blocksPort"`

	// Mempool
	MempoolSize         int      `json:"mempoolSize"`
	MempoolPayerSize    int      `json:"mempoolPayerSize"`
	MempoolExemptPayers []string `json:"mempoolExemptPayers"`

	// Order Book
	//
	// This is denoted as <asset 1>-<asset 2>
	//
	// TODO: add ability to denote min rate/min amount for tracking to avoid spam
	TrackedPairs []string `json:"trackedPairs"` // which asset ID pairs we care about

	// Misc
	TestMode    bool          `json:"testMode"` // makes gossip/building manual
	LogLevel    logging.Level `json:"logLevel"`
	Parallelism int           `json:"parallelism"`

	// State Sync
	StateSyncServerDelay time.Duration `json:"stateSyncServerDelay"` // for testing

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
	c.Parallelism = c.Config.GetParallelism()
	c.MempoolSize = c.Config.GetMempoolSize()
	c.MempoolPayerSize = c.Config.GetMempoolPayerSize()
	c.StateSyncServerDelay = c.Config.GetStateSyncServerDelay()
}

func (c *Config) GetLogLevel() logging.Level       { return c.LogLevel }
func (c *Config) GetTestMode() bool                { return c.TestMode }
func (c *Config) GetParallelism() int              { return c.Parallelism }
func (c *Config) GetMempoolSize() int              { return c.MempoolSize }
func (c *Config) GetMempoolPayerSize() int         { return c.MempoolPayerSize }
func (c *Config) GetMempoolExemptPayers() [][]byte { return c.parsedExemptPayers }
func (c *Config) GetDecisionsPort() uint16         { return c.DecisionsPort }
func (c *Config) GetBlocksPort() uint16            { return c.BlocksPort }
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
