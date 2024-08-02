// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"

	"github.com/ava-labs/avalanchego/utils/logging"
)

type Config struct {
	StoreTransactions bool          `json:"storeTransactions"`
	LogLevel          logging.Level `json:"logLevel"`
	ExportedBlockSubcribers string `json:"exportedBlockSubscribers"`
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
