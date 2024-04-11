// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

const (
	// These `codec` consts are defined here to avoid a circular dependency
	BoolLen   = 1
	ByteLen   = 1
	IDLen     = 32
	NodeIDLen = 20
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

	// MTU is the maximum practical packet size. On TCP, the max is technically 64
	// KB but Ethernet has a standard MTU of 1500 bytes. So, we use that with some space for padding.
	//
	// On AWS this can even be as low as 1300 bytes:
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/network_mtu.html
	//
	// If we go over this size, we see massive degradation from packet fragmentation:
	// https://en.wikipedia.org/wiki/IP_fragmentation
	//
	// The larget websocket header is 14 bytes:
	// https://www.openmymind.net/WebSocket-Framing-Masking-Fragmentation-and-More/
	MTU = 1_200

	// ClockSkewAllowance should be used whenever constructing an expiry time (outside of block production)
	// to account for potential skew between nodes (which will lead to drops).
	ClockSkewAllowance = 2 * MillisecondsPerSecond

	MaxUint8                  = ^uint8(0)
	MaxUint16                 = ^uint16(0)
	MaxUint8Offset            = 7
	MaxUint                   = ^uint(0)
	MaxInt                    = int(MaxUint >> 1)
	MaxUint64Offset           = 63
	MaxUint64                 = ^uint64(0)
	MillisecondsPerSecond     = 1000
	MillisecondsPerDecisecond = 100
)
