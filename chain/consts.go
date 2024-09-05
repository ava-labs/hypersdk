// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"
)

const (
	// FutureBound is used to ignore blocks that have a timestamp more than
	// [FutureBound] ahead of the current time (when verifying a block).
	//
	// This value should be (much) less than the value of [ProposerWindow], otherwise honest
	// nodes may not build during their allocated window to avoid increasing the skew of the
	// chain time.
	FutureBound = 1 * time.Second
	// MaxKeyDependencies must be greater than the maximum number of key dependencies
	// any single task could have when executing a task.
	MaxKeyDependencies = 100_000_000
)
