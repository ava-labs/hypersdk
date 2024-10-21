// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import "math"

const (
	// These `codec` consts are defined here to avoid a circular dependency
	BoolLen   = 1
	ByteLen   = 1
	IntLen    = 4
	Uint8Len  = 1
	Uint16Len = 2
	Uint32Len = 4
	Uint64Len = 8
	Int64Len  = 8

	// AvalancheGo imposes a limit of 2 MiB on the network, so we limit at
	// 2 MiB - ProposerVM header - Protobuf encoding overhead (we assume this is
	// no more than 50 KiB of overhead but is likely much less)
	NetworkSizeLimit = 2_044_723 // 1.95 MiB

	MaxUint8              = math.MaxUint8
	MaxUint16             = math.MaxUint16
	MaxUint8Offset        = 7
	MaxUint               = math.MaxUint
	MaxInt                = int(math.MaxUint >> 1)
	MaxUint64Offset       = 63
	MaxUint64             = math.MaxUint64
	MaxFloat64            = math.MaxFloat64
	MillisecondsPerSecond = 1000
	Decimals              = 9
)
