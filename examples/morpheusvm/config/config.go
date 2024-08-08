// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"encoding/json"
)

type Config struct {
	StoreTransactions bool `json:"storeTransactions"`
}

func New(b []byte) (*Config, error) {
	c := &Config{
		StoreTransactions: true,
	}

	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}
