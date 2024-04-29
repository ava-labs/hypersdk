// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"

	"github.com/ava-labs/hypersdk/keys"
)

const (
	// FutureBound is used to ignore blocks that have a timestamp more than
	// [FutureBound] ahead of the current time (when verifying a block).
	//
	// This value should be (much) less than the value of [ProposerWindow], otherwise honest
	// nodes may not build during their allocated window to avoid increasing the skew of the
	// chain time.
	FutureBound        = 1 * time.Second
	HeightKeyChunks    = 1
	TimestampKeyChunks = 1
	FeeKeyChunks       = 8 // 96 (per dimension) * 5 (num dimensions)

	// MaxKeyDependencies must be greater than the maximum number of key dependencies
	// any single task could have when executing a task.
	MaxKeyDependencies = 100_000_000
)

func HeightKey(prefix []byte) []byte {
	return keys.EncodeChunks(prefix, HeightKeyChunks)
}

func TimestampKey(prefix []byte) []byte {
	return keys.EncodeChunks(prefix, TimestampKeyChunks)
}

func FeeKey(prefix []byte) []byte {
	return keys.EncodeChunks(prefix, FeeKeyChunks)
}
