// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package context

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	r := require.New(t)
	c := make(Config)

	type VMConfig struct {
		TxFee uint64 `json:"txFee"`
	}
	configStr := `{"vm": {"txFee": 1000}}`
	r.NoError(json.Unmarshal([]byte(configStr), &c))
	vmConfig, err := GetConfig(c, "vm", VMConfig{})
	r.NoError(err)
	r.Equal(uint64(1000), vmConfig.TxFee)
}
