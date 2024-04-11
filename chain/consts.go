// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/keys"
)

const (
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
	PHeightKeyChunks      = 1
	TimestampKeyChunks    = 1
	FeeKeyChunks          = 8 // 96 (per dimension) * 5 (num dimensions)
	EpochKeyChunks        = 1
)

func HeightKey(prefix string) string {
	return keys.EncodeChunks(prefix, HeightKeyChunks)
}

func PHeightKey(prefix string) string {
	return keys.EncodeChunks(prefix, PHeightKeyChunks)
}

func TimestampKey(prefix string) string {
	return keys.EncodeChunks(prefix, TimestampKeyChunks)
}

func FeeKey(prefix string) string {
	return keys.EncodeChunks(prefix, FeeKeyChunks)
}

func EpochKey(prefix string) string {
	return keys.EncodeChunks(prefix, EpochKeyChunks)
}
