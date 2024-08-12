// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"

	"github.com/ava-labs/avalanchego/utils/logging"
)

type Config struct {
	IndexAll             bool          `json:"indexAll"`
	LogLevel                      logging.Level `json:"logLevel"`
	ExportedBlockSubcriberAddress string        `json:"exportedBlockSubscriberAddress"`
}

func New(b []byte) (*Config, error) {
	c := &Config{
		IndexAll: true,
		LogLevel:          logging.Info,
	}

	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}
