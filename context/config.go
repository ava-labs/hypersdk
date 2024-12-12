// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package context

import "encoding/json"

type Config map[string]json.RawMessage

func NewConfig(b []byte) (Config, error) {
	c := Config{}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
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

