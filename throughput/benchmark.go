// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

// Benchmark allows VMs to benchmark their performance, outside of the normal
// stats that are collected by default (e.g. TPS)
type Benchmark interface {
	LogOutputs(outputs [][]byte) error
	LogState()
}
