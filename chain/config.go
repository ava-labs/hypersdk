// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
)

type Config struct {
	TargetBuildDuration       time.Duration `json:"targetBuildDuration"`
	TransactionExecutionCores int           `json:"transactionExecutionCores"`
	StateFetchConcurrency     int           `json:"stateFetchConcurrency"`
	// We leave room for other block data to be included alongside the transactions
	TargetTxsSize int `json:"targetTxsSize"`
}

func NewDefaultConfig() Config {
	return Config{
		TargetBuildDuration:       100 * time.Millisecond,
		TransactionExecutionCores: 1,
		StateFetchConcurrency:     1,
		TargetTxsSize:             1.5 * units.MiB,
	}
}
