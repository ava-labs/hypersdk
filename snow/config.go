// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"encoding/json"

	"github.com/ava-labs/hypersdk/internal/trace"
)

type Config map[string]json.RawMessage

func NewConfig(b []byte) (Config, error) {
	c := Config{}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c Config) Get(key string) ([]byte, bool) {
	if val, ok := c[key]; ok {
		return val, true
	}
	return nil, false
}

func GetConfig[T any](c Config, key string, defaultConfig T) (T, error) {
	val, ok := c[key]
	if !ok {
		return defaultConfig, nil
	}

	var emptyConfig T
	if err := json.Unmarshal(val, &defaultConfig); err != nil {
		return emptyConfig, err
	}
	return defaultConfig, nil
}

type VMConfig struct {
	TraceConfig trace.Config `json:"traceConfig"`

	// Cache sizes
	ParsedBlockCacheSize     int `json:"parsedBlockCacheSize"`
	AcceptedBlockWindowCache int `json:"acceptedBlockWindowCache"`
}

func NewDefaultVMConfig() VMConfig {
	return VMConfig{
		TraceConfig:              trace.Config{Enabled: false},
		ParsedBlockCacheSize:     128,
		AcceptedBlockWindowCache: 128,
	}
}
