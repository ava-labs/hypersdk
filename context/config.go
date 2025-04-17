// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package context

import "encoding/json"

// Config represents a generic map-based configuration
// Each value is stored as a raw JSON value, which can be used to unmarshal into
// a specific config type T
type Config map[string]json.RawMessage

// NewEmptyConfig returns an empty config
func NewEmptyConfig() Config {
	return make(Config)
}

// NewConfig returns a config derived from b
// The contents of b are unmarshaled into the config
func NewConfig(b []byte) (Config, error) {
	c := Config{}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// GetRawConfig returns the value stored at key
func (c Config) GetRawConfig(key string) json.RawMessage {
	return c[key]
}

// GetConfig attempts to read the config stored at key and unmarshal it into a
// value of type T
// If the key does not exist or if the value cannot be unmarshaled into a Config
// value, the defaultConfig is returned
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

// SetConfig sets the value at key to value (must be JSON serializable)
func SetConfig[T any](c Config, key string, value T) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c[key] = b
	return nil
}
