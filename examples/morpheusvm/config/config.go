// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"

	"github.com/ava-labs/avalanchego/utils/logging"
)

type Config struct {
	// Misc
	StoreTransactions bool          `json:"storeTransactions"`
	TestMode          bool          `json:"testMode"` // makes gossip/building manual
	LogLevel          logging.Level `json:"logLevel"`
}

func New(b []byte) (*Config, error) {
	c := &Config{
		StoreTransactions: true,
		LogLevel:          logging.Info,
	}

	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}
