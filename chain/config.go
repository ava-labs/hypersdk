// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "time"

type ExecutionConfig struct {
	TargetBuildDuration       time.Duration `json:"targetBuildDuration"`
	TransactionExecutionCores int           `json:"transactionExecutionCores"`
	StateFetchConcurrency     int           `json:"stateFetchConcurrency"`
}

func NewDefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		TargetBuildDuration:       100 * time.Millisecond,
		TransactionExecutionCores: 1,
		StateFetchConcurrency:     1,
	}
}
