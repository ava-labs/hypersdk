// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/gossiper"
)

type Config struct {
	// Gossip
	GossipMaxSize       int   `json:"gossipMaxSize"`
	GossipProposerDiff  int   `json:"gossipProposerDiff"`
	GossipProposerDepth int   `json:"gossipProposerDepth"`
	NoGossipBuilderDiff int   `json:"noGossipBuilderDiff"`
	VerifyTimeout       int64 `json:"verifyTimeout"`

	// Order Book
	//
	// This is denoted as <asset 1>-<asset 2>
	MaxOrdersPerPair int      `json:"maxOrdersPerPair"`
	TrackedPairs     []string `json:"trackedPairs"` // which asset ID pairs we care about

	// Misc
	StoreTransactions bool          `json:"storeTransactions"`
	TestMode          bool          `json:"testMode"` // makes gossip/building manual
	LogLevel          logging.Level `json:"logLevel"`
}

func New(b []byte) (*Config, error) {
	gcfg := gossiper.DefaultProposerConfig()
	c := &Config{
		LogLevel:            logging.Info,
		GossipMaxSize:       gcfg.GossipMaxSize,
		GossipProposerDiff:  gcfg.GossipProposerDiff,
		GossipProposerDepth: gcfg.GossipProposerDepth,
		NoGossipBuilderDiff: gcfg.NoGossipBuilderDiff,
		VerifyTimeout:       gcfg.VerifyTimeout,
		StoreTransactions:   true,
		MaxOrdersPerPair:    1024,
	}

	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}
