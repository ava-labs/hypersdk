// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"encoding/json"
)

type Config struct {
	StoreTransactions bool `json:"storeTransactions"`
}

func newConfig(b []byte) (*Config, error) {
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
