// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	ReadBufferSize     = consts.NetworkSizeLimit
	WriteBufferSize    = consts.NetworkSizeLimit
	WriteWait          = 10 * time.Second
	PongWait           = 60 * time.Second
	PingPeriod         = (PongWait * 9) / 10
	MaxReadMessageSize = consts.NetworkSizeLimit
	// TargetWriteMessageSize attempts to send data at the maximum packet size. On TCP, this
	// is technically 64 KB but Ethernet has a standard MTU of 1500 bytes. So, we use
	// ~1 KB.
	//
	// On AWS this can even be as low as 1300 bytes:
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/network_mtu.html
	//
	// If we go over this size, we see massive degradation from packet fragmentation:
	// https://en.wikipedia.org/wiki/IP_fragmentation
	TargetWriteMessageSize = 1_200
	MaxWriteMessageSize    = 16 * units.MiB
	MaxMessageWait         = 50 * time.Millisecond
	MaxPendingMessages     = 1024
)
