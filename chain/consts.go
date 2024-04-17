// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/keys"
)

const (
	// FutureBound is used to ignore blocks that have a timestamp more than
	// [FutureBound] ahead of the current time (when verifying a block).
	//
	// This value should be (much) less than the value of [ProposerWindow], otherwise honest
	// nodes may not build during their allocated window to avoid increasing the skew of the
	// chain time.
	FutureBound = 1 * time.Second
	// MaxWarpMessageSize is the maximum size of a warp message.
	MaxWarpMessageSize = 256 * units.KiB
	// MaxWarpMessages is the maximum number of warp messages allows in a single
	// block.
	MaxWarpMessages = 64
	// MaxIncomingWarpChunks is the number of chunks stored for an incoming warp message.
	MaxIncomingWarpChunks = 0
	// MaxOutgoingWarpChunks is the max number of chunks that can be stored for an outgoing warp message.
	//
	// This is defined as a constant because storage of warp messages is handled by the hypersdk,
	// not the [Controller]. In this mechanism, we frequently query warp messages by TxID across
	// ranges (so, we can't expose a way to modify this over time).
	MaxOutgoingWarpChunks = 4
	HeightKeyChunks       = 1
	TimestampKeyChunks    = 1
	FeeKeyChunks          = 8 // 96 (per dimension) * 5 (num dimensions)

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
