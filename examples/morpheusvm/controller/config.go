// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"encoding/json"

	"github.com/ava-labs/hypersdk/api/ws"
)

var _ ws.WSConfig = (*Config)(nil)

type Config struct {
	StoreTransactions bool `json:"storeTransactions"`
	MaxPendingMessages int `json:"maxPendingMessages"`
}

func newConfig(b []byte) (*Config, error) {
	c := &Config{
		StoreTransactions: true,
		MaxPendingMessages: 10_000,
	}

	if len(b) > 0 {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c Config) GetMaxPendingMessages() int {
	return c.MaxPendingMessages
}
